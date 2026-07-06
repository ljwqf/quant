package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"log"
	"os"
	"time"
)

func main() {
	server := flag.String("server", "132.232.231.41:8514", "rsyslog TLS server address")
	caCert := flag.String("ca-cert", "certs/ca.crt", "CA certificate path")
	tag := flag.String("tag", "switch01", "syslog tag/hostname")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		args = []string{"test_logs/device_01.log"}
	}

	caPEM, err := os.ReadFile(*caCert)
	if err != nil {
		log.Fatalf("Failed to read CA cert: %v", err)
	}
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caPEM) {
		log.Fatal("Failed to parse CA cert")
	}

	tlsConfig := &tls.Config{
		RootCAs:            caPool,
		InsecureSkipVerify: false,
	}

	for _, file := range args {
		data, err := os.ReadFile(file)
		if err != nil {
			log.Printf("[SYSLOG] Failed to read %s: %v", file, err)
			continue
		}

		conn, err := tls.Dial("tcp", *server, tlsConfig)
		if err != nil {
			log.Printf("[SYSLOG] Connect %s failed: %v", *server, err)
			continue
		}

		lines := string(data)
		for _, line := range splitLines(lines) {
			if line == "" {
				continue
			}
			syslog := fmt.Sprintf("<%d>%s %s: %s\n", 134, time.Now().Format("Jan  2 15:04:05"), *tag, line)
			_, err = conn.Write([]byte(syslog))
			if err != nil {
				log.Printf("[SYSLOG] Write failed: %v", err)
				break
			}
		}

		conn.Close()
		fmt.Printf("[SYSLOG] Sent %s to %s\n", file, *server)
	}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
