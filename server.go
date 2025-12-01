package main

import (
	"crypto/tls"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/quic-go/quic-go/http3"
)

const (
	// Directory containing the 100+ static assets
	STATIC_DIR = "static"
	CERT_FILE  = "certs/server.crt"
	KEY_FILE   = "certs/server.key"
)

func main() {
	// Get ports from environment variables or use defaults
	h1Port := os.Getenv("H1_PORT")
	if h1Port == "" {
		h1Port = "8080"
	}
	h2Port := os.Getenv("H2_PORT")
	if h2Port == "" {
		h2Port = "8443"
	}
	h3Port := os.Getenv("H3_PORT")
	if h3Port == "" {
		h3Port = "8444"
	}

	// --- 1. Define the Static File Handler ---
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(STATIC_DIR)))

	var wg sync.WaitGroup

	// --- 2. Start HTTP/1.1 Server (plain HTTP) ---
	wg.Add(1)
	go func() {
		defer wg.Done()
		h1Addr := ":" + h1Port
		log.Printf("Starting HTTP/1.1 server on %s", h1Addr)
		if err := http.ListenAndServe(h1Addr, mux); err != nil {
			log.Printf("HTTP/1.1 server error: %v", err)
		}
	}()

	// Check if certificates exist before starting HTTPS servers
	if _, err := os.Stat(CERT_FILE); os.IsNotExist(err) {
		log.Printf("Warning: Certificate file %s not found. HTTP/2 and HTTP/3 servers will not start.", CERT_FILE)
		log.Printf("Generate certificates with: openssl req -x509 -newkey rsa:4096 -keyout %s -out %s -days 365 -nodes -subj \"/CN=localhost\"", KEY_FILE, CERT_FILE)
		wg.Wait()
		return
	}

	// Load TLS certificate
	cert, err := tls.LoadX509KeyPair(CERT_FILE, KEY_FILE)
	if err != nil {
		log.Fatalf("Failed to load TLS certificate: %v", err)
	}

	// --- 3. Start HTTP/2 Server (HTTPS with TLS, HTTP/2 only) ---
	wg.Add(1)
	go func() {
		defer wg.Done()
		h2Addr := ":" + h2Port
		log.Printf("Starting HTTP/2 server (TLS) on %s", h2Addr)

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			NextProtos:   []string{"h2"}, // HTTP/2 only
		}

		server := &http.Server{
			Addr:      h2Addr,
			Handler:   mux,
			TLSConfig: tlsConfig,
		}
		if err := server.ListenAndServeTLS("", ""); err != nil {
			log.Printf("HTTP/2 server error: %v", err)
		}
	}()

	// --- 4. Start HTTP/3 Server (QUIC) ---
	wg.Add(1)
	go func() {
		defer wg.Done()
		h3Addr := ":" + h3Port
		log.Printf("Starting HTTP/3 server (QUIC) on %s", h3Addr)

		h3Server := &http3.Server{
			Addr:      h3Addr,
			Handler:   mux,
			TLSConfig: http3.ConfigureTLSConfig(&tls.Config{Certificates: []tls.Certificate{cert}}),
		}
		if err := h3Server.ListenAndServe(); err != nil {
			log.Printf("HTTP/3 server error: %v", err)
		}
	}()

	log.Println("All servers started. Press Ctrl+C to stop.")
	log.Printf("  HTTP/1.1: http://localhost:%s", h1Port)
	log.Printf("  HTTP/2:   https://localhost:%s", h2Port)
	log.Printf("  HTTP/3:   https://localhost:%s (QUIC/UDP)", h3Port)

	wg.Wait()
}
