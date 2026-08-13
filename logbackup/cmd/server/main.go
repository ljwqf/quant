package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

type LogEntry struct {
	Source    string    `json:"source"`
	Filename  string    `json:"filename"`
	Size      int64     `json:"size"`
	Received  time.Time `json:"received"`
	LocalPath string    `json:"local_path"`
}

type Stats struct {
	mu            sync.Mutex
	TotalFiles    int64
	TotalBytes    int64
	LastEntry     *LogEntry
	RecentEntries []*LogEntry
}

func (s *Stats) Record(entry *LogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TotalFiles++
	s.TotalBytes += entry.Size
	s.LastEntry = entry
	s.RecentEntries = append(s.RecentEntries, entry)
	if len(s.RecentEntries) > 100 {
		s.RecentEntries = s.RecentEntries[len(s.RecentEntries)-100:]
	}
}

func main() {
	addr := flag.String("addr", ":8443", "TLS listen address")
	caCert := flag.String("ca-cert", "certs/ca.crt", "CA certificate path")
	serverCert := flag.String("server-cert", "certs/server.crt", "Server certificate path")
	serverKey := flag.String("server-key", "certs/server.key", "Server key path")
	logDir := flag.String("log-dir", "./logs_received", "Directory to store received logs")
	statsFile := flag.String("stats-file", "./stats.json", "Stats output path")
	flag.Parse()

	// Load CA cert
	caPEM, err := os.ReadFile(*caCert)
	if err != nil {
		log.Fatalf("Failed to read CA cert: %v", err)
	}
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caPEM) {
		log.Fatal("Failed to parse CA cert")
	}

	// Load server cert
	cert, err := tls.LoadX509KeyPair(*serverCert, *serverKey)
	if err != nil {
		log.Fatalf("Failed to load server cert: %v", err)
	}

	// TLS config: require client certs signed by our CA
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caPool,
		MinVersion:   tls.VersionTLS12,
	}

	if err := os.MkdirAll(*logDir, 0755); err != nil {
		log.Fatalf("Failed to create log dir: %v", err)
	}

	stats := &Stats{}

	ln, err := tls.Listen("tcp", *addr, tlsConfig)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	defer ln.Close()

	log.Printf("[SERVER] Listening on %s (TLS, mTLS enabled)", *addr)
	log.Printf("[SERVER] CA cert: %s", *caCert)
	log.Printf("[SERVER] Log storage: %s", *logDir)

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				data, err := json.MarshalIndent(stats, "", "  ")
				if err != nil {
					log.Printf("[SERVER] Failed to marshal stats: %v", err)
					continue
				}
				if err := os.WriteFile(*statsFile, data, 0644); err != nil {
					log.Printf("[SERVER] Failed to write stats: %v", err)
				}
			case <-sigCh:
				log.Println("[SERVER] Shutting down...")
				data, err := json.MarshalIndent(stats, "", "  ")
				if err != nil {
					log.Printf("[SERVER] Failed to marshal stats on shutdown: %v", err)
				} else {
					if err := os.WriteFile(*statsFile, data, 0644); err != nil {
						log.Printf("[SERVER] Failed to write stats on shutdown: %v", err)
					}
				}
				os.Exit(0)
			}
		}
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("[SERVER] Accept error: %v", err)
			continue
		}
		go handleConn(conn, *logDir, stats)
	}
}

func handleConn(conn net.Conn, logDir string, stats *Stats) {
	defer conn.Close()

	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		log.Printf("[SERVER] Non-TLS connection")
		return
	}

	// Perform TLS handshake to populate peer certificates
	if err := tlsConn.Handshake(); err != nil {
		log.Printf("[SERVER] TLS handshake failed: %v", err)
		return
	}

	// Get client CN from cert
	state := tlsConn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		log.Printf("[SERVER] No peer certificates")
		return
	}
	cert := state.PeerCertificates[0]
	clientCN := cert.Subject.CommonName
	log.Printf("[SERVER] Connected: %s (CN=%s)", conn.RemoteAddr(), clientCN)

	// Read header line: "FILENAME <name>\n"
	header, err := readLine(conn, 1024)
	if err != nil {
		log.Printf("[SERVER] Read header failed: %v", err)
		return
	}

	if !strings.HasPrefix(header, "FILENAME ") {
		fmt.Fprintf(conn, "ERROR: expected FILENAME header\n")
		return
	}
	filename := strings.TrimSpace(strings.TrimPrefix(header, "FILENAME "))
	filename = filepath.Base(filename) // sanitize

	// Create destination file
	dest := filepath.Join(logDir, fmt.Sprintf("%s_%s", time.Now().Format("20060102_150405"), filename))
	f, err := os.Create(dest)
	if err != nil {
		fmt.Fprintf(conn, "ERROR: %v\n", err)
		return
	}
	defer f.Close()

	// Read file content until EOF
	n, err := io.Copy(f, conn)
	if err != nil && err != io.EOF {
		log.Printf("[SERVER] Read error: %v", err)
	}

	log.Printf("[SERVER] Received: %s (%d bytes) from %s", filename, n, clientCN)

	entry := &LogEntry{
		Source:    clientCN,
		Filename:  filename,
		Size:      n,
		Received:  time.Now(),
		LocalPath: dest,
	}
	stats.Record(entry)

	fmt.Fprintf(conn, "OK: %d bytes received\n", n)
}

func readLine(r io.Reader, max int) (string, error) {
	buf := make([]byte, 0, max)
	for {
		b := make([]byte, 1)
		_, err := r.Read(b)
		if err != nil {
			return string(buf), err
		}
		if b[0] == '\n' {
			return string(buf), nil
		}
		buf = append(buf, b[0])
		if len(buf) >= max {
			return string(buf), fmt.Errorf("header too long")
		}
	}
}
