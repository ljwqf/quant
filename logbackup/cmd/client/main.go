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
	server := flag.String("server", "127.0.0.1:8443", "Server address")
	caCert := flag.String("ca-cert", "certs/ca.crt", "CA certificate path")
	clientCert := flag.String("client-cert", "certs/client.crt", "Client certificate path")
	clientKey := flag.String("client-key", "certs/client.key", "Client key path")
	genTest := flag.Int("gen-test", 0, "Generate N test log files and send them")
	flag.Parse()

	args := flag.Args()
	if *genTest > 0 {
		generateTestFiles(*genTest)
		args = listTestFiles(*genTest)
	} else if len(args) == 0 {
		fmt.Println("Usage: log-client <logfile1> [logfile2] ...")
		fmt.Println("       log-client -gen-test <count>")
		os.Exit(1)
	}

	// Load CA
	caPEM, err := os.ReadFile(*caCert)
	if err != nil {
		log.Fatalf("Failed to read CA cert: %v", err)
	}
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caPEM) {
		log.Fatal("Failed to parse CA cert")
	}

	// Load client cert
	cert, err := tls.LoadX509KeyPair(*clientCert, *clientKey)
	if err != nil {
		log.Fatalf("Failed to load client cert: %v", err)
	}

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            caPool,
		InsecureSkipVerify: false,
	}

	for _, file := range args {
		if err := sendFile(*server, file, tlsConfig); err != nil {
			log.Printf("[CLIENT] Failed to send %s: %v", file, err)
		} else {
			log.Printf("[CLIENT] Sent %s successfully", file)
		}
	}
}

func sendFile(server string, filePath string, tlsConfig *tls.Config) error {
	conn, err := tls.Dial("tcp", server, tlsConfig)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close()

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	// Send header
	_, err = fmt.Fprintf(conn, "FILENAME %s\n", filePath)
	if err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	// Send file content
	_, err = conn.Write(data)
	if err != nil {
		return fmt.Errorf("write data: %w", err)
	}

	conn.CloseWrite()

	// Read response
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	fmt.Printf("  Server response: %s", buf[:n])
	return nil
}

func generateTestFiles(count int) {
	os.MkdirAll("./test_logs", 0755)

	sampleLogs := []string{
		// Cisco IOS style
		`*Mar  1 00:01:15.123: %SYS-5-CONFIG_I: Configured from console by admin on vty0 (192.168.1.100)
*Mar  1 00:01:16.456: %LINK-3-UPDOWN: Interface GigabitEthernet0/1, changed state to up
*Mar  1 00:02:01.789: %OSPF-5-ADJCHG: Process 1, Nbr 10.0.0.2 on GigabitEthernet0/1 from LOADING to FULL, Loading Done
*Mar  1 00:02:15.012: %BGP-5-ADJCHG: neighbor 10.0.0.2 WENT from Idle to Established
*Mar  1 00:03:00.345: %LINEPROTO-5-UPDOWN: Line protocol on Interface GigabitEthernet0/1, changed state to up
*Mar  1 00:03:22.678: %IP-4-DUPADDR: Duplicate address 192.168.1.1 on GigabitEthernet0/1
*Mar  1 00:04:10.901: %SEC-6-IPACCESSLOGP: list ACL_IN denied tcp 10.10.10.5(45678) -> 192.168.1.100(22), 1 packet
*Mar  1 00:05:00.234: %SYS-6-SYSLOG_STOP: Stopped logging to buffered
`,
		// Firewall style (Palo Alto / Fortinet)
		`2026-05-10 08:00:01 traffic permit src=192.168.1.50 dst=8.8.8.8 proto=udp sport=54321 dport=53 app=dns
2026-05-10 08:00:02 traffic deny src=10.0.0.99 dst=192.168.1.1 proto=tcp sport=12345 dport=22 app=ssh
2026-05-10 08:00:03 threat high src=203.0.113.50 dst=192.168.1.100 proto=tcp sport=443 dport=8080 app=exploit.cve.2024.1234
2026-05-10 08:00:04 traffic permit src=192.168.1.10 dst=10.0.0.5 proto=tcp sport=49152 dport=443 app=ssl
2026-05-10 08:00:05 traffic permit src=192.168.1.20 dst=10.0.0.5 proto=tcp sport=49153 dport=443 app=ssl
2026-05-10 08:00:06 threat critical src=198.51.100.10 dst=192.168.1.50 proto=tcp sport=80 dport=51234 app=ransomware.dropper
2026-05-10 08:00:07 traffic deny src=172.16.0.100 dst=192.168.1.0/24 proto=icmp sport=0 dport=0 app=ping
2026-05-10 08:00:08 traffic permit src=192.168.1.1 dst=10.0.0.1 proto=tcp sport=50000 dport=22 app=ssh
`,
		// Linux syslog style
		`May 10 08:00:01 switch01 sshd[12345]: Accepted publickey for admin from 192.168.1.100 port 52134 ssh2
May 10 08:00:02 switch01 kernel: [45678.901] eth0: link up, 1000Mbps, full-duplex
May 10 08:00:03 switch01 systemd[1]: Started OpenSSH server.
May 10 08:00:04 switch01 ntpd[567]: ntpd: time sync OK, offset +0.0012s
May 10 08:00:05 switch01 CRON[678]: (root) CMD (/usr/local/bin/health-check.sh)
May 10 08:00:06 switch01 kernel: [45679.012] WARNING: CPU temperature above threshold, cpu clock throttled
May 10 08:00:07 switch01 sshd[12346]: Failed password for root from 203.0.113.50 port 43210 ssh2
May 10 08:00:08 switch01 sshd[12346]: message repeated 5 times
May 10 08:00:09 switch01 sudo: admin : TTY=pts/0 ; PWD=/root ; USER=root ; COMMAND=/sbin/reboot
`,
	}

	for i := 0; i < count; i++ {
		name := fmt.Sprintf("./test_logs/device_%02d.log", i+1)
		content := sampleLogs[i%len(sampleLogs)]
		// Add timestamps to make it realistic
		header := fmt.Sprintf("# Log backup test - device_%02d - %s\n", i+1, time.Now().Format("2006-01-02 15:04:05"))
		os.WriteFile(name, []byte(header+content), 0644)
		fmt.Printf("  Generated: %s\n", name)
	}
}

func listTestFiles(count int) []string {
	files := make([]string, count)
	for i := 0; i < count; i++ {
		files[i] = fmt.Sprintf("./test_logs/device_%02d.log", i+1)
	}
	return files
}
