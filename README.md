# HTTP Protocol Benchmark Lab

A benchmark tool for testing HTTP/1.1, HTTP/2, and HTTP/3 protocol performance by loading multiple static assets.

## Prerequisites

- **Go** 1.21+ installed
- **OpenSSL** installed (for generating self-signed certificates)
- **k6** installed (for running load tests)

### Installing k6

**Windows (using winget):**

```powershell

winget install k6 --source winget

```

**macOS:**

```bash
brew install k6
```

**Linux (Debian/Ubuntu):**

```bash
sudo apt-get update
sudo apt-get install k6
```

## Setup Instructions

### Step 1: Download Test Images

First, download the test images (100 images: 5 large + 95 small):

```bash
go run ./cmd/image_downloader
```

This will download images to `static/images/` folder.

### Step 2: Generate SSL Certificates

Generate self-signed certificates for HTTPS testing:

```bash
# Create certs directory if it doesn't exist
mkdir -p certs

# Generate a self-signed certificate (valid for 365 days)
openssl req -x509 -newkey rsa:4096 -keyout certs/server.key -out certs/server.crt -days 365 -nodes -subj "/CN=localhost"
```

### Step 3: Start the Server

Start the HTTP server (supports HTTP/1.1, HTTP/2, and HTTP/3):

**macOS/Linux:**

```bash
go run .
```

**Windows (PowerShell):**

```powershell
go run .
```

The server will start on:
- **HTTP/1.1**: `http://localhost:8080` (plain HTTP)
- **HTTP/1.1 + HTTP/2**: `https://localhost:8443` (TLS)
- **HTTP/3**: `https://localhost:8443` (QUIC/UDP)

You can customize ports with environment variables:

**macOS/Linux:**

```bash
HTTP_PORT=8080 HTTPS_PORT=8443 go run .
```

**Windows (PowerShell):**

```powershell
$env:HTTP_PORT="8080"; $env:HTTPS_PORT="8443"; go run .
```

Open <http://localhost:8080> in your browser to verify the page loads with all images.

### Step 4: Run Benchmarks with k6

Run the benchmark script:

```bash
# Basic test against localhost
k6 run http_benchmark.js -e TARGET_HOST='http://localhost:8080'

# Test with specific protocol version
k6 run http_benchmark.js -e TARGET_HOST='https://localhost:8080' -e K6_PROTOCOL='2'
```

**Environment Variables:**
- `TARGET_HOST`: The server URL to test (default: `https://localhost`)
- `K6_PROTOCOL`: HTTP protocol version to test (`1.1`, `2`, or `3`)

## Project Structure

```
├── cmd/
│   └── image_downloader/    # Tool to download test images
│       └── main.go
├── static/
│   ├── index.html           # Main benchmark page
│   ├── app.js               # JavaScript for loading images
│   ├── style.css            # Styles
│   └── images/              # Downloaded test images (gitignored)
├── certs/                   # SSL certificates (gitignored)
├── server.go                # Main HTTP server
├── http_benchmark.js        # k6 benchmark script
├── Dockerfile               # Container build file
└── README.md
```

## Quick Start Summary

**macOS/Linux:**

```bash
# 1. Download test images
go run ./cmd/image_downloader

# 2. Generate SSL certificates
mkdir -p certs
openssl req -x509 -newkey rsa:4096 -keyout certs/server.key -out certs/server.crt -days 365 -nodes -subj "/CN=localhost"

# 3. Start the server
go run .

# 4. Run benchmark (in another terminal)
k6 run http_benchmark.js -e TARGET_HOST='http://localhost:8080'
k6 run http_benchmark.js -e TARGET_HOST='https://localhost:8443' -e K6_PROTOCOL='2'
```

**Windows (PowerShell):**

```powershell
# 1. Download test images
go run ./cmd/image_downloader

# 2. Generate SSL certificates (requires OpenSSL installed, e.g., via Git Bash or chocolatey)
New-Item -ItemType Directory -Force -Path certs
openssl req -x509 -newkey rsa:4096 -keyout certs/server.key -out certs/server.crt -days 365 -nodes -subj "/CN=localhost"

# 3. Start the server
go run .

# 4. Run benchmark (in another terminal)
k6 run http_benchmark.js -e TARGET_HOST='http://localhost:8080'
k6 run http_benchmark.js -e TARGET_HOST='https://localhost:8443' -e K6_PROTOCOL='2'
```

## License

MIT License
