package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"os"
	"sort"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"golang.org/x/net/http2"
)

// TimingResult holds all timing metrics for a single request
type TimingResult struct {
	Protocol        string
	DNSLookup       time.Duration
	TCPConnect      time.Duration
	TLSHandshake    time.Duration
	TimeToFirstByte time.Duration
	ContentTransfer time.Duration
	TotalTime       time.Duration
	StatusCode      int
	Error           error
}

// BenchmarkResult holds aggregated results for a protocol
type BenchmarkResult struct {
	Protocol           string
	TotalRequests      int
	SuccessfulRequests int
	FailedRequests     int

	// Timing averages
	AvgDNSLookup       time.Duration
	AvgTCPConnect      time.Duration
	AvgTLSHandshake    time.Duration
	AvgTimeToFirstByte time.Duration
	AvgContentTransfer time.Duration
	AvgTotalTime       time.Duration

	// Timing percentiles
	P50TotalTime time.Duration
	P95TotalTime time.Duration
	P99TotalTime time.Duration

	// Min/Max
	MinTotalTime time.Duration
	MaxTotalTime time.Duration

	// Throughput
	RequestsPerSecond float64
}

func main() {
	url := flag.String("url", "https://http-evolution-benchmark.zjkung1123.workers.dev/api/protocol", "URL to benchmark")
	requests := flag.Int("n", 100, "Number of requests per protocol")
	concurrency := flag.Int("c", 10, "Number of concurrent requests")
	protocols := flag.String("p", "h1,h2,h3", "Protocols to test (comma-separated: h1,h2,h3)")
	outputDir := flag.String("o", "", "Output directory to save JSON results (optional)")
	flag.Parse()

	fmt.Println("╔══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║           HTTP Protocol Benchmark Tool (H1, H2, H3)              ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════╝")
	fmt.Printf("\nTarget URL: %s\n", *url)
	fmt.Printf("Requests per protocol: %d\n", *requests)
	fmt.Printf("Concurrency: %d\n", *concurrency)
	fmt.Printf("Protocols: %s\n\n", *protocols)

	results := make(map[string]*BenchmarkResult)

	// Parse protocols
	protocolList := parseProtocols(*protocols)

	for _, proto := range protocolList {
		fmt.Printf("Testing %s...\n", proto)
		result := runBenchmark(*url, proto, *requests, *concurrency)
		results[proto] = result
		fmt.Printf("  Completed: %d/%d successful\n", result.SuccessfulRequests, result.TotalRequests)
	}

	// Print results
	printResults(results)

	// Save results to file if output directory is specified
	if *outputDir != "" {
		saveResults(results, *outputDir, *url, *requests, *concurrency)
	}
}

func parseProtocols(protocols string) []string {
	var result []string
	for _, p := range []string{"h1", "h2", "h3"} {
		for _, input := range splitString(protocols, ',') {
			if input == p {
				result = append(result, p)
				break
			}
		}
	}
	return result
}

func splitString(s string, sep rune) []string {
	var result []string
	current := ""
	for _, c := range s {
		if c == sep {
			if current != "" {
				result = append(result, current)
			}
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func runBenchmark(url, protocol string, numRequests, concurrency int) *BenchmarkResult {
	client := createClient(protocol)
	defer closeClient(client, protocol)

	results := make([]TimingResult, 0, numRequests)
	var mu sync.Mutex
	var wg sync.WaitGroup

	semaphore := make(chan struct{}, concurrency)
	startTime := time.Now()

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		semaphore <- struct{}{}

		go func(reqNum int) {
			defer wg.Done()
			defer func() { <-semaphore }()

			result := makeRequest(client, url, protocol)
			if result.Error != nil && reqNum == 0 {
				fmt.Printf("  Error (sample): %v\n", result.Error)
			}
			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}(i)
	}

	wg.Wait()
	totalDuration := time.Since(startTime)

	return aggregateResults(protocol, results, totalDuration)
}

func createClient(protocol string) *http.Client {
	switch protocol {
	case "h1":
		return &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: false,
				},
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     90 * time.Second,
				TLSNextProto:        make(map[string]func(authority string, c *tls.Conn) http.RoundTripper), // Disable HTTP/2
			},
			Timeout: 30 * time.Second,
		}

	case "h2":
		transport := &http2.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
			},
			AllowHTTP: false,
		}
		return &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		}

	case "h3":
		transport := &http3.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
			},
			QUICConfig: &quic.Config{
				MaxIdleTimeout:  30 * time.Second,
				KeepAlivePeriod: 10 * time.Second,
			},
		}
		return &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		}

	default:
		return http.DefaultClient
	}
}

func closeClient(client *http.Client, protocol string) {
	if protocol == "h3" {
		if transport, ok := client.Transport.(*http3.Transport); ok {
			transport.Close()
		}
	}
}

func makeRequest(client *http.Client, url, protocol string) TimingResult {
	result := TimingResult{Protocol: protocol}

	var dnsStart, dnsEnd time.Time
	var connectStart, connectEnd time.Time
	var tlsStart, tlsEnd time.Time
	var firstByteTime time.Time
	requestStart := time.Now()

	// Create trace for HTTP/1.1 and HTTP/2
	trace := &httptrace.ClientTrace{
		DNSStart: func(info httptrace.DNSStartInfo) {
			dnsStart = time.Now()
		},
		DNSDone: func(info httptrace.DNSDoneInfo) {
			dnsEnd = time.Now()
		},
		ConnectStart: func(network, addr string) {
			connectStart = time.Now()
		},
		ConnectDone: func(network, addr string, err error) {
			connectEnd = time.Now()
		},
		TLSHandshakeStart: func() {
			tlsStart = time.Now()
		},
		TLSHandshakeDone: func(state tls.ConnectionState, err error) {
			tlsEnd = time.Now()
		},
		GotFirstResponseByte: func() {
			firstByteTime = time.Now()
		},
	}

	ctx := httptrace.WithClientTrace(context.Background(), trace)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		result.Error = err
		return result
	}

	resp, err := client.Do(req)
	if err != nil {
		result.Error = err
		result.TotalTime = time.Since(requestStart)
		return result
	}
	defer resp.Body.Close()

	// Read body to ensure full transfer
	_, err = io.Copy(io.Discard, resp.Body)
	if err != nil {
		result.Error = err
	}

	requestEnd := time.Now()

	result.StatusCode = resp.StatusCode
	result.TotalTime = requestEnd.Sub(requestStart)

	// Calculate timing metrics
	if !dnsEnd.IsZero() && !dnsStart.IsZero() {
		result.DNSLookup = dnsEnd.Sub(dnsStart)
	}
	if !connectEnd.IsZero() && !connectStart.IsZero() {
		result.TCPConnect = connectEnd.Sub(connectStart)
	}
	if !tlsEnd.IsZero() && !tlsStart.IsZero() {
		result.TLSHandshake = tlsEnd.Sub(tlsStart)
	}
	if !firstByteTime.IsZero() {
		result.TimeToFirstByte = firstByteTime.Sub(requestStart)
	}
	if !firstByteTime.IsZero() {
		result.ContentTransfer = requestEnd.Sub(firstByteTime)
	}

	return result
}

func aggregateResults(protocol string, results []TimingResult, totalDuration time.Duration) *BenchmarkResult {
	br := &BenchmarkResult{
		Protocol:      protocol,
		TotalRequests: len(results),
		MinTotalTime:  time.Hour, // Start with a large value
	}

	var successfulResults []TimingResult
	var totalTimes []time.Duration

	for _, r := range results {
		if r.Error != nil {
			br.FailedRequests++
			continue
		}
		br.SuccessfulRequests++
		successfulResults = append(successfulResults, r)
		totalTimes = append(totalTimes, r.TotalTime)

		// Accumulate for averages
		br.AvgDNSLookup += r.DNSLookup
		br.AvgTCPConnect += r.TCPConnect
		br.AvgTLSHandshake += r.TLSHandshake
		br.AvgTimeToFirstByte += r.TimeToFirstByte
		br.AvgContentTransfer += r.ContentTransfer
		br.AvgTotalTime += r.TotalTime

		// Min/Max
		if r.TotalTime < br.MinTotalTime {
			br.MinTotalTime = r.TotalTime
		}
		if r.TotalTime > br.MaxTotalTime {
			br.MaxTotalTime = r.TotalTime
		}
	}

	// Calculate averages
	if br.SuccessfulRequests > 0 {
		count := time.Duration(br.SuccessfulRequests)
		br.AvgDNSLookup /= count
		br.AvgTCPConnect /= count
		br.AvgTLSHandshake /= count
		br.AvgTimeToFirstByte /= count
		br.AvgContentTransfer /= count
		br.AvgTotalTime /= count
	}

	// Calculate percentiles
	if len(totalTimes) > 0 {
		sort.Slice(totalTimes, func(i, j int) bool {
			return totalTimes[i] < totalTimes[j]
		})
		br.P50TotalTime = percentile(totalTimes, 50)
		br.P95TotalTime = percentile(totalTimes, 95)
		br.P99TotalTime = percentile(totalTimes, 99)
	}

	// Calculate throughput
	if totalDuration > 0 {
		br.RequestsPerSecond = float64(br.SuccessfulRequests) / totalDuration.Seconds()
	}

	return br
}

func percentile(sorted []time.Duration, p int) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	index := (p * len(sorted)) / 100
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

func printResults(results map[string]*BenchmarkResult) {
	fmt.Println("\n╔══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                        BENCHMARK RESULTS                         ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════╝")

	// Print detailed results for each protocol
	for _, proto := range []string{"h1", "h2", "h3"} {
		r, ok := results[proto]
		if !ok {
			continue
		}

		protoName := map[string]string{
			"h1": "HTTP/1.1",
			"h2": "HTTP/2",
			"h3": "HTTP/3 (QUIC)",
		}[proto]

		fmt.Printf("\n┌─────────────────────────────────────────────────────────────────┐\n")
		fmt.Printf("│ %s%-63s%s │\n", "\033[1;36m", protoName, "\033[0m")
		fmt.Printf("├─────────────────────────────────────────────────────────────────┤\n")
		fmt.Printf("│ Requests: %d total, %d successful, %d failed                    \n",
			r.TotalRequests, r.SuccessfulRequests, r.FailedRequests)
		fmt.Printf("│ Throughput: %.2f req/sec                                        \n", r.RequestsPerSecond)
		fmt.Printf("├─────────────────────────────────────────────────────────────────┤\n")
		fmt.Printf("│ %-25s %s                                      \n", "DNS Lookup:", formatDuration(r.AvgDNSLookup))
		fmt.Printf("│ %-25s %s                                      \n", "TCP Connect:", formatDuration(r.AvgTCPConnect))
		fmt.Printf("│ %-25s %s                                      \n", "TLS Handshake:", formatDuration(r.AvgTLSHandshake))
		fmt.Printf("│ %-25s %s                                      \n", "Time to First Byte:", formatDuration(r.AvgTimeToFirstByte))
		fmt.Printf("│ %-25s %s                                      \n", "Content Transfer:", formatDuration(r.AvgContentTransfer))
		fmt.Printf("├─────────────────────────────────────────────────────────────────┤\n")
		fmt.Printf("│ %-25s %s                                      \n", "Avg Total Time:", formatDuration(r.AvgTotalTime))
		fmt.Printf("│ %-25s %s                                      \n", "Min Total Time:", formatDuration(r.MinTotalTime))
		fmt.Printf("│ %-25s %s                                      \n", "Max Total Time:", formatDuration(r.MaxTotalTime))
		fmt.Printf("│ %-25s %s                                      \n", "P50 (Median):", formatDuration(r.P50TotalTime))
		fmt.Printf("│ %-25s %s                                      \n", "P95:", formatDuration(r.P95TotalTime))
		fmt.Printf("│ %-25s %s                                      \n", "P99:", formatDuration(r.P99TotalTime))
		fmt.Printf("└─────────────────────────────────────────────────────────────────┘\n")
	}

	// Print comparison table
	printComparisonTable(results)

	// Print JSON output for programmatic use
	printJSONResults(results)
}

func printComparisonTable(results map[string]*BenchmarkResult) {
	fmt.Println("\n╔══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                      COMPARISON SUMMARY                          ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════╝\n")

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Metric\tHTTP/1.1\tHTTP/2\tHTTP/3")
	fmt.Fprintln(w, "──────\t────────\t──────\t──────")

	getVal := func(proto string, fn func(*BenchmarkResult) string) string {
		if r, ok := results[proto]; ok {
			return fn(r)
		}
		return "N/A"
	}

	fmt.Fprintf(w, "Avg Total Time\t%s\t%s\t%s\n",
		getVal("h1", func(r *BenchmarkResult) string { return formatDuration(r.AvgTotalTime) }),
		getVal("h2", func(r *BenchmarkResult) string { return formatDuration(r.AvgTotalTime) }),
		getVal("h3", func(r *BenchmarkResult) string { return formatDuration(r.AvgTotalTime) }))

	fmt.Fprintf(w, "Time to First Byte\t%s\t%s\t%s\n",
		getVal("h1", func(r *BenchmarkResult) string { return formatDuration(r.AvgTimeToFirstByte) }),
		getVal("h2", func(r *BenchmarkResult) string { return formatDuration(r.AvgTimeToFirstByte) }),
		getVal("h3", func(r *BenchmarkResult) string { return formatDuration(r.AvgTimeToFirstByte) }))

	fmt.Fprintf(w, "TLS Handshake\t%s\t%s\t%s\n",
		getVal("h1", func(r *BenchmarkResult) string { return formatDuration(r.AvgTLSHandshake) }),
		getVal("h2", func(r *BenchmarkResult) string { return formatDuration(r.AvgTLSHandshake) }),
		getVal("h3", func(r *BenchmarkResult) string { return formatDuration(r.AvgTLSHandshake) }))

	fmt.Fprintf(w, "Throughput (req/s)\t%s\t%s\t%s\n",
		getVal("h1", func(r *BenchmarkResult) string { return fmt.Sprintf("%.2f", r.RequestsPerSecond) }),
		getVal("h2", func(r *BenchmarkResult) string { return fmt.Sprintf("%.2f", r.RequestsPerSecond) }),
		getVal("h3", func(r *BenchmarkResult) string { return fmt.Sprintf("%.2f", r.RequestsPerSecond) }))

	fmt.Fprintf(w, "P99 Latency\t%s\t%s\t%s\n",
		getVal("h1", func(r *BenchmarkResult) string { return formatDuration(r.P99TotalTime) }),
		getVal("h2", func(r *BenchmarkResult) string { return formatDuration(r.P99TotalTime) }),
		getVal("h3", func(r *BenchmarkResult) string { return formatDuration(r.P99TotalTime) }))

	fmt.Fprintf(w, "Success Rate\t%s\t%s\t%s\n",
		getVal("h1", func(r *BenchmarkResult) string {
			return fmt.Sprintf("%.1f%%", float64(r.SuccessfulRequests)/float64(r.TotalRequests)*100)
		}),
		getVal("h2", func(r *BenchmarkResult) string {
			return fmt.Sprintf("%.1f%%", float64(r.SuccessfulRequests)/float64(r.TotalRequests)*100)
		}),
		getVal("h3", func(r *BenchmarkResult) string {
			return fmt.Sprintf("%.1f%%", float64(r.SuccessfulRequests)/float64(r.TotalRequests)*100)
		}))

	w.Flush()
}

func printJSONResults(results map[string]*BenchmarkResult) {
	fmt.Println("\n╔══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                        JSON OUTPUT                               ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════╝\n")

	// Convert to JSON-friendly format
	jsonResults := make(map[string]interface{})
	for proto, r := range results {
		jsonResults[proto] = map[string]interface{}{
			"protocol":                r.Protocol,
			"total_requests":          r.TotalRequests,
			"successful_requests":     r.SuccessfulRequests,
			"failed_requests":         r.FailedRequests,
			"avg_dns_lookup_ms":       float64(r.AvgDNSLookup) / float64(time.Millisecond),
			"avg_tcp_connect_ms":      float64(r.AvgTCPConnect) / float64(time.Millisecond),
			"avg_tls_handshake_ms":    float64(r.AvgTLSHandshake) / float64(time.Millisecond),
			"avg_ttfb_ms":             float64(r.AvgTimeToFirstByte) / float64(time.Millisecond),
			"avg_content_transfer_ms": float64(r.AvgContentTransfer) / float64(time.Millisecond),
			"avg_total_time_ms":       float64(r.AvgTotalTime) / float64(time.Millisecond),
			"min_total_time_ms":       float64(r.MinTotalTime) / float64(time.Millisecond),
			"max_total_time_ms":       float64(r.MaxTotalTime) / float64(time.Millisecond),
			"p50_total_time_ms":       float64(r.P50TotalTime) / float64(time.Millisecond),
			"p95_total_time_ms":       float64(r.P95TotalTime) / float64(time.Millisecond),
			"p99_total_time_ms":       float64(r.P99TotalTime) / float64(time.Millisecond),
			"requests_per_second":     r.RequestsPerSecond,
		}
	}

	jsonBytes, _ := json.MarshalIndent(jsonResults, "", "  ")
	fmt.Println(string(jsonBytes))
}

func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0ms"
	}
	if d < time.Millisecond {
		return fmt.Sprintf("%.2fµs", float64(d)/float64(time.Microsecond))
	}
	if d < time.Second {
		return fmt.Sprintf("%.2fms", float64(d)/float64(time.Millisecond))
	}
	return fmt.Sprintf("%.2fs", float64(d)/float64(time.Second))
}

// RTT estimation using TCP connect time for H1/H2 or QUIC handshake for H3
func estimateRTT(results map[string]*BenchmarkResult) {
	fmt.Println("\n╔══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                      RTT ESTIMATION                              ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════╝\n")

	for proto, r := range results {
		var rtt time.Duration
		switch proto {
		case "h1", "h2":
			// TCP connect approximates 1 RTT
			rtt = r.AvgTCPConnect
		case "h3":
			// For QUIC, the handshake is approximately 1-RTT for resumption
			// or 2-RTT for full handshake, we estimate using TLS time
			rtt = r.AvgTLSHandshake / 2
		}
		fmt.Printf("%s estimated RTT: %s\n", proto, formatDuration(rtt))
	}
}

func init() {
	// Disable connection pooling verification for cleaner output
	net.DefaultResolver.PreferGo = true
}

func saveResults(results map[string]*BenchmarkResult, outputDir, url string, requests, concurrency int) {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Printf("Error creating output directory: %v\n", err)
		return
	}

	// Create timestamp for filename
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("benchmark_%s.json", timestamp)
	filepath := outputDir + "/" + filename

	// Build output structure
	output := map[string]interface{}{
		"metadata": map[string]interface{}{
			"url":         url,
			"requests":    requests,
			"concurrency": concurrency,
			"timestamp":   time.Now().Format(time.RFC3339),
		},
		"results": map[string]interface{}{},
	}

	resultsMap := output["results"].(map[string]interface{})
	for proto, r := range results {
		resultsMap[proto] = map[string]interface{}{
			"protocol":                r.Protocol,
			"total_requests":          r.TotalRequests,
			"successful_requests":     r.SuccessfulRequests,
			"failed_requests":         r.FailedRequests,
			"avg_dns_lookup_ms":       float64(r.AvgDNSLookup) / float64(time.Millisecond),
			"avg_tcp_connect_ms":      float64(r.AvgTCPConnect) / float64(time.Millisecond),
			"avg_tls_handshake_ms":    float64(r.AvgTLSHandshake) / float64(time.Millisecond),
			"avg_ttfb_ms":             float64(r.AvgTimeToFirstByte) / float64(time.Millisecond),
			"avg_content_transfer_ms": float64(r.AvgContentTransfer) / float64(time.Millisecond),
			"avg_total_time_ms":       float64(r.AvgTotalTime) / float64(time.Millisecond),
			"min_total_time_ms":       float64(r.MinTotalTime) / float64(time.Millisecond),
			"max_total_time_ms":       float64(r.MaxTotalTime) / float64(time.Millisecond),
			"p50_total_time_ms":       float64(r.P50TotalTime) / float64(time.Millisecond),
			"p95_total_time_ms":       float64(r.P95TotalTime) / float64(time.Millisecond),
			"p99_total_time_ms":       float64(r.P99TotalTime) / float64(time.Millisecond),
			"requests_per_second":     r.RequestsPerSecond,
		}
	}

	jsonBytes, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling JSON: %v\n", err)
		return
	}

	if err := os.WriteFile(filepath, jsonBytes, 0644); err != nil {
		fmt.Printf("Error writing file: %v\n", err)
		return
	}

	fmt.Printf("\n✅ Results saved to: %s\n", filepath)
}
