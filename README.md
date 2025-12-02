# HTTP Protocol Benchmark Lab

A comprehensive benchmark tool for testing and comparing HTTP/1.1, HTTP/2, and HTTP/3 (QUIC) protocol performance. Includes a Go-based benchmark client and Python visualization tools.

## Features

- 🚀 **HTTP Protocol Benchmarking**: Test HTTP/1.1, HTTP/2, and HTTP/3 performance
- 📊 **Detailed Timing Metrics**: DNS lookup, TCP connect, TLS handshake, TTFB, content transfer
- 📈 **Python Visualization**: Generate charts comparing protocol performance
- ☁️ **Cloudflare Workers**: Deploy benchmark endpoints to Cloudflare's edge network
- 🐳 **Docker Support**: Containerized deployment ready

---

## Prerequisites

### Required

- **Go** 1.21+ - [Download Go](https://golang.org/dl/)
- **Python** 3.10+ - [Download Python](https://www.python.org/downloads/)
- **uv** (Python package manager) - [Install uv](https://docs.astral.sh/uv/getting-started/installation/)

### Optional

- **OpenSSL** - For generating self-signed certificates (local testing)
- **Node.js** 18+ - For Cloudflare Workers deployment
- **Wrangler CLI** - `npm install -g wrangler`

---

## Installation

### 1. Clone the Repository

```bash
git clone https://github.com/ZJKung/panel4-lab.git
cd panel4-lab
```

### 2. Install Go Dependencies

```bash
go mod download
```

### 3. Build the Benchmark Tool

**Windows (PowerShell):**

```powershell
go build -o httpbench.exe ./cmd/httpbench
```

**macOS/Linux:**

```bash
go build -o httpbench ./cmd/httpbench
```

### 4. Install Python Dependencies

Using **uv** (recommended):

```bash
# Install uv if not already installed
# Windows: powershell -c "irm https://astral.sh/uv/install.ps1 | iex"
# macOS/Linux: curl -LsSf https://astral.sh/uv/install.sh | sh

# Sync dependencies
uv sync
```

Or using pip:

```bash
pip install matplotlib numpy
```

---

## Usage

### Running the HTTP Benchmark

The `httpbench` tool measures HTTP/1.1, HTTP/2, and HTTP/3 performance with detailed timing metrics.

**Basic Usage:**

```bash
# Windows
.\httpbench.exe -url "https://http-evolution-benchmark.zjkung1123.workers.dev/api/protocol" -n 50 -c 10 -p "h1,h2,h3" -o "results"

# macOS/Linux
./httpbench -url "https://http-evolution-benchmark.zjkung1123.workers.dev/api/protocol" -n 50 -c 10 -p "h1,h2,h3" -o "results"
```

**Command Line Options:**

| Flag   | Description                         | Default                               |
| ------ | ----------------------------------- | ------------------------------------- |
| `-url` | Target URL to benchmark             | `https://...workers.dev/api/protocol` |
| `-n`   | Number of requests per protocol     | `100`                                 |
| `-c`   | Number of concurrent requests       | `10`                                  |
| `-p`   | Protocols to test (comma-separated) | `h1,h2,h3`                            |
| `-o`   | Output directory for JSON results   | (none)                                |

**Examples:**

```bash
# Test only HTTP/2 and HTTP/3
.\httpbench.exe -url "https://example.com" -p "h2,h3" -n 100

# High concurrency test
.\httpbench.exe -url "https://example.com" -c 50 -n 500

# Save results to file
.\httpbench.exe -url "https://example.com" -o "results"
```

### Visualizing Results

Generate charts from benchmark results using Python:

```bash
# Using uv (recommended)
uv run python scripts/visualize_benchmark.py results/

# Or using Python directly
python scripts/visualize_benchmark.py results/
```

**Available Charts:**

- **Timing Breakdown**: Stacked bar chart showing DNS, TCP, TLS, TTFB, and content transfer times
- **Latency Comparison**: Average latency by protocol
- **Throughput Comparison**: Requests per second by protocol
- **Response Time Range**: Min/Max/Avg response times with percentiles
- **Dashboard**: Combined overview of all metrics

Charts are saved to `results/charts/` directory.

---

## Project Structure

```
├── cmd/
│   ├── httpbench/           # HTTP benchmark tool
│   │   └── main.go
│   └── image_downloader/    # Tool to download test images
│       └── main.go
├── cloudflare/              # Cloudflare Workers deployment
│   ├── src/
│   │   └── worker.js        # Worker script
│   ├── wrangler.toml        # Wrangler configuration
│   └── package.json
├── scripts/
│   └── visualize_benchmark.py  # Python visualization
├── static/
│   ├── index.html           # Static benchmark page
│   ├── app.js               # JavaScript for loading images
│   ├── style.css            # Styles
│   └── images/              # Test images (gitignored)
├── results/                 # Benchmark results (gitignored)
├── certs/                   # SSL certificates (gitignored)
├── server.go                # Local HTTP server
├── pyproject.toml           # Python project configuration
├── go.mod                   # Go module file
├── Dockerfile               # Container build file
├── fly.toml                 # Fly.io deployment config
└── README.md
```text

```

---

## Local Development

### Running the Local Server

For local testing with HTTP/1.1, HTTP/2, and HTTP/3:

#### Step 1: Download Test Images

```bash
go run ./cmd/image_downloader
```

#### Step 2: Generate SSL Certificates

```bash
# Create certs directory
mkdir -p certs  # Linux/macOS
New-Item -ItemType Directory -Force -Path certs  # Windows

# Generate self-signed certificate
openssl req -x509 -newkey rsa:4096 -keyout certs/server.key -out certs/server.crt -days 365 -nodes -subj "/CN=localhost"
```

#### Step 3: Start the Server

```bash
go run .
```

The server will start on:

- **HTTP/1.1**: `http://localhost:8080`
- **HTTP/1.1 + HTTP/2**: `https://localhost:8443`
- **HTTP/3**: `https://localhost:8443` (QUIC/UDP)

---

## Cloudflare Workers Deployment

Deploy the benchmark API to Cloudflare's edge network:

```bash
cd cloudflare

# Install dependencies
npm install

# Login to Cloudflare
npx wrangler login

# Deploy
npx wrangler deploy
```

**API Endpoints:**

| Endpoint                 | Description                            |
| ------------------------ | -------------------------------------- |
| `/`                      | Static HTML page                       |
| `/api/protocol`          | Returns the HTTP protocol version used |
| `/api/small`             | Returns 1KB payload                    |
| `/api/medium`            | Returns 10KB payload                   |
| `/api/large`             | Returns 100KB payload                  |
| `/api/latency?delay=100` | Returns after specified delay (ms)     |

---

## Troubleshooting

### HTTP/3 Connection Failures

If HTTP/3 tests fail with timeout errors:

1. **Check VPN/Firewall**: Cloudflare WARP and some VPNs block UDP port 443
2. **Disable WARP**: Turn off Cloudflare WARP if enabled
3. **Check UDP connectivity**: HTTP/3 uses QUIC over UDP

### Python Visualization Errors

If visualization fails:

1. **Empty results folder**: Run the benchmark first with `-o results`
2. **Missing dependencies**: Run `uv sync` or `pip install matplotlib numpy`

---

## Quick Start Summary

```bash
# 1. Build the benchmark tool
go build -o httpbench.exe ./cmd/httpbench  # Windows
go build -o httpbench ./cmd/httpbench      # macOS/Linux

# 2. Run benchmark and save results
.\httpbench.exe -url "https://http-evolution-benchmark.zjkung1123.workers.dev/api/protocol" -n 50 -c 10 -p "h1,h2,h3" -o "results"

# 3. Visualize results
uv run python scripts/visualize_benchmark.py results/
```

---

## License

MIT License
