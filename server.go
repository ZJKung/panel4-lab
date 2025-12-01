package main

import (
	"log"
	"net/http"
	"os"
	// We remove the quic-go/http3 dependency as the GCP LB handles H3.
)

const (
	// Directory containing the 100+ static assets
	STATIC_DIR = "static"
)

func main() {
	// Cloud Run expects the port to be read from the PORT environment variable
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// --- 1. Define the Static File Handler ---
	// This serves all your HTML, images, CSS, JS from the './static' directory.
	// We use http.StripPrefix to ensure the FileServer handles paths correctly.
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(STATIC_DIR)))

	// --- 2. Start HTTP Server (H1/H2/H3 traffic comes encrypted from the LB) ---
	addr := ":" + port
	log.Printf("Starting HTTP server (internal) on %s, listening for LB traffic.", addr)

	// Cloud Run requires the server to listen on HTTP (not HTTPS/TLS)
	// The Global External Load Balancer handles the external HTTPS/TLS.
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("HTTP server failed: %v", err)
	}
}