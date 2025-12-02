package main

import (
	"crypto/tls"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

const (
	STATIC_DIR = "static"
	CERT_FILE  = "certs/server.crt"
	KEY_FILE   = "certs/server.key"
)

// noCacheMiddleware adds Cache-Control: no-store header to all responses
func noCacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func main() {
	mode := os.Getenv("DEPLOY_MODE")

	switch mode {
	case "flyio":
		runFlyioMode()
	case "cloudrun":
		runCloudRunMode()
	default:
		runBenchmarkMode()
	}
}

// runFlyioMode - Single port for Fly.io with h2c (HTTP/2 cleartext) support
// Fly.io handles TLS termination and supports HTTP/1.1, HTTP/2, and HTTP/3 automatically
func runFlyioMode() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()

	// Health check endpoint for Fly.io
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Protocol info endpoint - helps verify which protocol is being used
	mux.HandleFunc("/protocol", func(w http.ResponseWriter, r *http.Request) {
		// r.Proto shows the backend protocol (between Fly proxy and this server)
		backendProto := r.Proto

		// Fly-Client-IP and other headers can help identify the edge connection
		// Fly.io sets Fly-Forwarded-Proto for the original protocol
		flyForwardedProto := r.Header.Get("Fly-Forwarded-Proto")

		// X-Forwarded-Proto is the standard header
		xForwardedProto := r.Header.Get("X-Forwarded-Proto")

		// Check for Via header which may contain protocol info
		via := r.Header.Get("Via")

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.Write([]byte(`{` +
			`"backend_protocol":"` + backendProto + `",` +
			`"fly_forwarded_proto":"` + flyForwardedProto + `",` +
			`"x_forwarded_proto":"` + xForwardedProto + `",` +
			`"via":"` + via + `"` +
			`}`))
	})

	// Wrap file server with no-cache middleware
	mux.Handle("/", noCacheMiddleware(http.FileServer(http.Dir(STATIC_DIR))))

	// Wrap handler with h2c to support HTTP/2 cleartext (unencrypted HTTP/2)
	// This allows Fly.io's proxy to communicate with our server over HTTP/2
	h2s := &http2.Server{}
	h2cHandler := h2c.NewHandler(mux, h2s)

	addr := ":" + port
	log.Printf("[Fly.io Mode] Starting HTTP/1.1 + h2c server on %s", addr)
	log.Printf("Fly.io handles TLS and supports HTTP/1.1, HTTP/2 & HTTP/3 at the edge")
	log.Printf("Backend supports both HTTP/1.1 and HTTP/2 (h2c) connections from Fly proxy")
	log.Printf("Cache-Control: no-store enabled for all responses")

	server := &http.Server{
		Addr:    addr,
		Handler: h2cHandler,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// runCloudRunMode - Single port HTTP server for Cloud Run
func runCloudRunMode() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	// Wrap file server with no-cache middleware
	mux.Handle("/", noCacheMiddleware(http.FileServer(http.Dir(STATIC_DIR))))

	addr := ":" + port
	log.Printf("[Cloud Run Mode] Starting HTTP server on %s", addr)
	log.Printf("Cloud Run handles TLS and supports HTTP/1.1 & HTTP/2 automatically")
	log.Printf("Note: HTTP/3 is NOT supported on Cloud Run")
	log.Printf("Cache-Control: no-store enabled for all responses")

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// runBenchmarkMode - Multi-port server for local/VM benchmarking H1/H2/H3
func runBenchmarkMode() {
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

	mux := http.NewServeMux()
	// Wrap file server with no-cache middleware
	mux.Handle("/", noCacheMiddleware(http.FileServer(http.Dir(STATIC_DIR))))

	var wg sync.WaitGroup

	// --- HTTP/1.1 Server (plain HTTP) ---
	wg.Add(1)
	go func() {
		defer wg.Done()
		h1Addr := ":" + h1Port
		log.Printf("Starting HTTP/1.1 server on %s", h1Addr)
		if err := http.ListenAndServe(h1Addr, mux); err != nil {
			log.Printf("HTTP/1.1 server error: %v", err)
		}
	}()

	// Check if certificates exist
	if _, err := os.Stat(CERT_FILE); os.IsNotExist(err) {
		log.Printf("Warning: Certificate file %s not found. HTTP/2 and HTTP/3 servers will not start.", CERT_FILE)
		log.Printf("Generate certificates with: openssl req -x509 -newkey rsa:4096 -keyout %s -out %s -days 365 -nodes -subj \"/CN=localhost\"", KEY_FILE, CERT_FILE)
		wg.Wait()
		return
	}

	cert, err := tls.LoadX509KeyPair(CERT_FILE, KEY_FILE)
	if err != nil {
		log.Fatalf("Failed to load TLS certificate: %v", err)
	}

	// --- HTTP/2 Server (HTTPS with TLS) ---
	wg.Add(1)
	go func() {
		defer wg.Done()
		h2Addr := ":" + h2Port
		log.Printf("Starting HTTP/2 server (TLS) on %s", h2Addr)

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			NextProtos:   []string{"h2"},
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

	// --- HTTP/3 Server (QUIC) with 0-RTT enabled ---
	wg.Add(1)
	go func() {
		defer wg.Done()
		h3Addr := ":" + h3Port
		log.Printf("Starting HTTP/3 server (QUIC) with 0-RTT on %s", h3Addr)

		// Configure QUIC with 0-RTT (early data) support
		quicConfig := &quic.Config{
			Allow0RTT: true, // Enable 0-RTT for faster connection resumption
		}

		h3Server := &http3.Server{
			Addr:       h3Addr,
			Handler:    mux,
			TLSConfig:  http3.ConfigureTLSConfig(&tls.Config{Certificates: []tls.Certificate{cert}}),
			QUICConfig: quicConfig,
		}
		if err := h3Server.ListenAndServe(); err != nil {
			log.Printf("HTTP/3 server error: %v", err)
		}
	}()

	log.Println("[Benchmark Mode] All servers started. Press Ctrl+C to stop.")
	log.Printf("  HTTP/1.1: http://localhost:%s", h1Port)
	log.Printf("  HTTP/2:   https://localhost:%s", h2Port)
	log.Printf("  HTTP/3:   https://localhost:%s (QUIC/UDP with 0-RTT)", h3Port)
	log.Printf("  Cache-Control: no-store enabled for all responses")

	wg.Wait()
}
