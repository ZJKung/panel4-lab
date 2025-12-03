#!/bin/bash
# HTTP Protocol Benchmark Script
# Compares HTTP/1.1, HTTP/2, and HTTP/3 performance
#
# This script uses curl to measure real page load times including all resources
# Results are saved to results/ directory in JSON format

set -e

# Configuration
BASE_URL="${BASE_URL:-https://http-evolution-benchmark.zjkung1123.workers.dev}"
CURL_BIN="${CURL_BIN:-/opt/homebrew/opt/curl/bin/curl}"
ITERATIONS="${ITERATIONS:-5}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RESULTS_DIR="${SCRIPT_DIR}/results"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
RESULT_FILE="${RESULTS_DIR}/benchmark_${TIMESTAMP}.json"

# Create results directory if it doesn't exist
mkdir -p "$RESULTS_DIR"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Initialize result arrays
declare -a SINGLE_H1_RUNS SINGLE_H1_TTFB
declare -a SINGLE_H2_RUNS SINGLE_H2_TTFB
declare -a SINGLE_H3_RUNS SINGLE_H3_TTFB
declare -a LARGE_H1_RUNS LARGE_H1_TTFB
declare -a LARGE_H2_RUNS LARGE_H2_TTFB
declare -a LARGE_H3_RUNS LARGE_H3_TTFB
declare -a XLARGE_H1_RUNS XLARGE_H1_TTFB
declare -a XLARGE_H2_RUNS XLARGE_H2_TTFB
declare -a XLARGE_H3_RUNS XLARGE_H3_TTFB
CONCURRENT_H1="" CONCURRENT_H2="" CONCURRENT_H3=""
FULLPAGE_H1="" FULLPAGE_H2="" FULLPAGE_H3=""

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}HTTP Protocol Benchmark${NC}"
echo -e "${BLUE}========================================${NC}"
echo "Target: $BASE_URL"
echo "Iterations per protocol: $ITERATIONS"
echo "Results will be saved to: $RESULT_FILE"
echo ""

# Check curl version
echo -e "${YELLOW}Curl Version:${NC}"
CURL_VERSION=$($CURL_BIN --version | head -1)
echo "$CURL_VERSION"
echo ""

# Timing format for curl
TIMING_FORMAT='time_namelookup: %{time_namelookup}s\ntime_connect: %{time_connect}s\ntime_appconnect: %{time_appconnect}s\ntime_pretransfer: %{time_pretransfer}s\ntime_starttransfer: %{time_starttransfer}s\ntime_total: %{time_total}s\nspeed_download: %{speed_download} bytes/sec\nsize_download: %{size_download} bytes\nhttp_code: %{http_code}\nhttp_version: %{http_version}\n'

# Function to run benchmark for a single URL and store results
benchmark_url_with_storage() {
    local protocol=$1
    local url=$2
    local curl_flags=$3
    local label=$4
    local runs_array_name=$5
    local ttfb_array_name=$6

    echo -e "\n${YELLOW}[$protocol] $label${NC}"
    echo "URL: $url"

    local total_time=0
    local total_ttfb=0

    for i in $(seq 1 $ITERATIONS); do
        result=$($CURL_BIN $curl_flags -o /dev/null -s -w "$TIMING_FORMAT" "$url" 2>&1)

        time_total=$(echo "$result" | grep "time_total" | cut -d' ' -f2 | tr -d 's')
        time_starttransfer=$(echo "$result" | grep "time_starttransfer" | cut -d' ' -f2 | tr -d 's')
        http_version=$(echo "$result" | grep "http_version" | cut -d' ' -f2)

        total_time=$(echo "$total_time + $time_total" | bc)
        total_ttfb=$(echo "$total_ttfb + $time_starttransfer" | bc)

        # Store results
        eval "${runs_array_name}+=($time_total)"
        eval "${ttfb_array_name}+=($time_starttransfer)"

        printf "  Run %d: Total=%.3fs, TTFB=%.3fs, Version=%s\n" $i $time_total $time_starttransfer $http_version
    done

    avg_time=$(echo "scale=3; $total_time / $ITERATIONS" | bc)
    avg_ttfb=$(echo "scale=3; $total_ttfb / $ITERATIONS" | bc)

    echo -e "${GREEN}  Average: Total=${avg_time}s, TTFB=${avg_ttfb}s${NC}"
}

# Function to benchmark single requests
benchmark_single_request() {
    echo -e "\n${BLUE}=== Single Request Benchmark (API endpoint) ===${NC}"

    benchmark_url_with_storage "HTTP/1.1" "$BASE_URL/api/protocol" "--http1.1" "Protocol Check" "SINGLE_H1_RUNS" "SINGLE_H1_TTFB"
    benchmark_url_with_storage "HTTP/2" "$BASE_URL/api/protocol" "--http2" "Protocol Check" "SINGLE_H2_RUNS" "SINGLE_H2_TTFB"
    benchmark_url_with_storage "HTTP/3" "$BASE_URL/api/protocol" "--http3" "Protocol Check" "SINGLE_H3_RUNS" "SINGLE_H3_TTFB"
}

# Function to benchmark large file download
benchmark_large_file() {
    echo -e "\n${BLUE}=== Large File Download Benchmark ===${NC}"
    echo "Downloading a single large image (~1MB)"

    benchmark_url_with_storage "HTTP/1.1" "$BASE_URL/images/image_001_large_2400x2400.jpg" "--http1.1" "Large Image" "LARGE_H1_RUNS" "LARGE_H1_TTFB"
    benchmark_url_with_storage "HTTP/2" "$BASE_URL/images/image_001_large_2400x2400.jpg" "--http2" "Large Image" "LARGE_H2_RUNS" "LARGE_H2_TTFB"
    benchmark_url_with_storage "HTTP/3" "$BASE_URL/images/image_001_large_2400x2400.jpg" "--http3" "Large Image" "LARGE_H3_RUNS" "LARGE_H3_TTFB"
}

# Function to benchmark 100MB file download
benchmark_xlarge_file() {
    echo -e "\n${BLUE}=== Extra Large File Download Benchmark (100MB) ===${NC}"
    echo "Downloading a 100MB binary file"

    benchmark_url_with_storage "HTTP/1.1" "$BASE_URL/100mb.bin" "--http1.1" "100MB Binary" "XLARGE_H1_RUNS" "XLARGE_H1_TTFB"
    benchmark_url_with_storage "HTTP/2" "$BASE_URL/100mb.bin" "--http2" "100MB Binary" "XLARGE_H2_RUNS" "XLARGE_H2_TTFB"
    benchmark_url_with_storage "HTTP/3" "$BASE_URL/100mb.bin" "--http3" "100MB Binary" "XLARGE_H3_RUNS" "XLARGE_H3_TTFB"
}

# Function to benchmark with multiple concurrent connections
benchmark_concurrent() {
    echo -e "\n${BLUE}=== Concurrent Requests Benchmark ===${NC}"
    echo "Downloading 10 images concurrently"

    for protocol in "http1.1" "http2" "http3"; do
        case $protocol in
            "http1.1") curl_flag="--http1.1"; label="HTTP/1.1" ;;
            "http2") curl_flag="--http2"; label="HTTP/2" ;;
            "http3") curl_flag="--http3"; label="HTTP/3" ;;
        esac

        echo -e "\n${YELLOW}[$label] 10 Concurrent Image Downloads${NC}"

        start_time=$(date +%s.%N)

        # Download 10 small images concurrently
        for i in $(seq 6 15); do
            idx=$(printf "%03d" $i)
            $CURL_BIN $curl_flag -s -o /dev/null "$BASE_URL/images/image_${idx}_small_300x300.jpg" &
        done
        wait

        end_time=$(date +%s.%N)
        total_time=$(echo "$end_time - $start_time" | bc | awk '{printf "%.6f", $0}')

        # Store result
        case $protocol in
            "http1.1") CONCURRENT_H1=$total_time ;;
            "http2") CONCURRENT_H2=$total_time ;;
            "http3") CONCURRENT_H3=$total_time ;;
        esac

        echo -e "${GREEN}  Total time: ${total_time}s${NC}"
    done
}

# Function to benchmark full page load
benchmark_full_page() {
    echo -e "\n${BLUE}=== Full Page Load Benchmark ===${NC}"
    echo "This downloads the HTML page and extracts all image URLs"

    for protocol in "http1.1" "http2" "http3"; do
        case $protocol in
            "http1.1") curl_flag="--http1.1"; label="HTTP/1.1" ;;
            "http2") curl_flag="--http2"; label="HTTP/2" ;;
            "http3") curl_flag="--http3"; label="HTTP/3" ;;
        esac

        echo -e "\n${YELLOW}[$label] Full Page Load (100 images)${NC}"

        page_url="$BASE_URL/benchmark.html"
        start_time=$(date +%s.%N)

        html=$($CURL_BIN $curl_flag -s "$page_url")
        images=$(echo "$html" | grep -oE 'src="/images/[^"]+' | sed 's/src="//')

        echo "$images" | while read img; do
            $CURL_BIN $curl_flag -s -o /dev/null "$BASE_URL$img" &
        done
        wait

        end_time=$(date +%s.%N)
        total_time=$(echo "$end_time - $start_time" | bc | awk '{printf "%.6f", $0}')

        # Store result
        case $protocol in
            "http1.1") FULLPAGE_H1=$total_time ;;
            "http2") FULLPAGE_H2=$total_time ;;
            "http3") FULLPAGE_H3=$total_time ;;
        esac

        echo -e "${GREEN}  Total time for HTML + 100 images: ${total_time}s${NC}"
    done
}

# Function to convert bash array to JSON array
array_to_json() {
    local arr=("$@")
    local json="["
    local first=true
    for val in "${arr[@]}"; do
        if [ "$first" = true ]; then
            first=false
        else
            json+=", "
        fi
        json+="$val"
    done
    json+="]"
    echo "$json"
}

# Function to calculate average (with leading zero for valid JSON)
calc_avg() {
    local arr=("$@")
    local sum=0
    local count=${#arr[@]}
    for val in "${arr[@]}"; do
        sum=$(echo "$sum + $val" | bc)
    done
    local result=$(echo "scale=3; $sum / $count" | bc)
    # Add leading zero if result starts with a dot
    if [[ "$result" == .* ]]; then
        result="0$result"
    fi
    echo "$result"
}

# Function to save results to JSON
save_results() {
    echo -e "\n${BLUE}Saving results to JSON...${NC}"

    # Calculate averages
    local single_h1_avg=$(calc_avg "${SINGLE_H1_RUNS[@]}")
    local single_h1_ttfb_avg=$(calc_avg "${SINGLE_H1_TTFB[@]}")
    local single_h2_avg=$(calc_avg "${SINGLE_H2_RUNS[@]}")
    local single_h2_ttfb_avg=$(calc_avg "${SINGLE_H2_TTFB[@]}")
    local single_h3_avg=$(calc_avg "${SINGLE_H3_RUNS[@]}")
    local single_h3_ttfb_avg=$(calc_avg "${SINGLE_H3_TTFB[@]}")

    local large_h1_avg=$(calc_avg "${LARGE_H1_RUNS[@]}")
    local large_h1_ttfb_avg=$(calc_avg "${LARGE_H1_TTFB[@]}")
    local large_h2_avg=$(calc_avg "${LARGE_H2_RUNS[@]}")
    local large_h2_ttfb_avg=$(calc_avg "${LARGE_H2_TTFB[@]}")
    local large_h3_avg=$(calc_avg "${LARGE_H3_RUNS[@]}")
    local large_h3_ttfb_avg=$(calc_avg "${LARGE_H3_TTFB[@]}")

    local xlarge_h1_avg=$(calc_avg "${XLARGE_H1_RUNS[@]}")
    local xlarge_h1_ttfb_avg=$(calc_avg "${XLARGE_H1_TTFB[@]}")
    local xlarge_h2_avg=$(calc_avg "${XLARGE_H2_RUNS[@]}")
    local xlarge_h2_ttfb_avg=$(calc_avg "${XLARGE_H2_TTFB[@]}")
    local xlarge_h3_avg=$(calc_avg "${XLARGE_H3_RUNS[@]}")
    local xlarge_h3_ttfb_avg=$(calc_avg "${XLARGE_H3_TTFB[@]}")

    cat > "$RESULT_FILE" << EOF
{
  "benchmark_date": "$(date -Iseconds)",
  "target": "$BASE_URL",
  "curl_version": "$CURL_VERSION",
  "iterations": $ITERATIONS,
  "results": {
    "single_request": {
      "description": "API endpoint latency test",
      "endpoint": "/api/protocol",
      "http1_1": {
        "runs": $(array_to_json "${SINGLE_H1_RUNS[@]}"),
        "ttfb": $(array_to_json "${SINGLE_H1_TTFB[@]}"),
        "avg_total": $single_h1_avg,
        "avg_ttfb": $single_h1_ttfb_avg
      },
      "http2": {
        "runs": $(array_to_json "${SINGLE_H2_RUNS[@]}"),
        "ttfb": $(array_to_json "${SINGLE_H2_TTFB[@]}"),
        "avg_total": $single_h2_avg,
        "avg_ttfb": $single_h2_ttfb_avg
      },
      "http3": {
        "runs": $(array_to_json "${SINGLE_H3_RUNS[@]}"),
        "ttfb": $(array_to_json "${SINGLE_H3_TTFB[@]}"),
        "avg_total": $single_h3_avg,
        "avg_ttfb": $single_h3_ttfb_avg
      }
    },
    "large_file": {
      "description": "Large image download (~1MB)",
      "endpoint": "/images/image_001_large_2400x2400.jpg",
      "http1_1": {
        "runs": $(array_to_json "${LARGE_H1_RUNS[@]}"),
        "ttfb": $(array_to_json "${LARGE_H1_TTFB[@]}"),
        "avg_total": $large_h1_avg,
        "avg_ttfb": $large_h1_ttfb_avg
      },
      "http2": {
        "runs": $(array_to_json "${LARGE_H2_RUNS[@]}"),
        "ttfb": $(array_to_json "${LARGE_H2_TTFB[@]}"),
        "avg_total": $large_h2_avg,
        "avg_ttfb": $large_h2_ttfb_avg
      },
      "http3": {
        "runs": $(array_to_json "${LARGE_H3_RUNS[@]}"),
        "ttfb": $(array_to_json "${LARGE_H3_TTFB[@]}"),
        "avg_total": $large_h3_avg,
        "avg_ttfb": $large_h3_ttfb_avg
      }
    },
    "xlarge_file": {
      "description": "Extra large binary download (100MB)",
      "endpoint": "/100mb.bin",
      "file_size_mb": 100,
      "http1_1": {
        "runs": $(array_to_json "${XLARGE_H1_RUNS[@]}"),
        "ttfb": $(array_to_json "${XLARGE_H1_TTFB[@]}"),
        "avg_total": $xlarge_h1_avg,
        "avg_ttfb": $xlarge_h1_ttfb_avg
      },
      "http2": {
        "runs": $(array_to_json "${XLARGE_H2_RUNS[@]}"),
        "ttfb": $(array_to_json "${XLARGE_H2_TTFB[@]}"),
        "avg_total": $xlarge_h2_avg,
        "avg_ttfb": $xlarge_h2_ttfb_avg
      },
      "http3": {
        "runs": $(array_to_json "${XLARGE_H3_RUNS[@]}"),
        "ttfb": $(array_to_json "${XLARGE_H3_TTFB[@]}"),
        "avg_total": $xlarge_h3_avg,
        "avg_ttfb": $xlarge_h3_ttfb_avg
      }
    },
    "concurrent": {
      "description": "10 concurrent image downloads (multiplexing test)",
      "num_images": 10,
      "http1_1": {
        "total_time": $CONCURRENT_H1
      },
      "http2": {
        "total_time": $CONCURRENT_H2
      },
      "http3": {
        "total_time": $CONCURRENT_H3
      }
    },
    "full_page": {
      "description": "Full page load (HTML + 20 images)",
      "endpoint": "/benchmark",
      "num_images": 20,
      "http1_1": {
        "total_time": $FULLPAGE_H1
      },
      "http2": {
        "total_time": $FULLPAGE_H2
      },
      "http3": {
        "total_time": $FULLPAGE_H3
      }
    }
  }
}
EOF

    echo -e "${GREEN}Results saved to: $RESULT_FILE${NC}"

    # Also create a symlink to latest result
    ln -sf "benchmark_${TIMESTAMP}.json" "${RESULTS_DIR}/latest.json"
    echo -e "${GREEN}Symlink created: ${RESULTS_DIR}/latest.json${NC}"
}

# Run all benchmarks
echo -e "${BLUE}Starting benchmarks...${NC}"

# 1. Single request (latency focused)
benchmark_single_request

# 2. Large file (throughput focused)
benchmark_large_file

# 3. Extra large file (100MB throughput test)
benchmark_xlarge_file

# 4. Concurrent requests (multiplexing focused)
benchmark_concurrent

# 5. Full page load simulation
benchmark_full_page

# Save results to JSON
save_results

echo -e "\n${BLUE}========================================${NC}"
echo -e "${GREEN}Benchmark Complete!${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""
echo "To visualize results, run:"
echo "  python visualize_benchmark.py results/latest.json"
