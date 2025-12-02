import http from "k6/http";
import { check, group, sleep } from "k6";
import { Trend, Counter } from "k6/metrics";

// HTTP/3 support via xk6-http3 extension
// Build k6 with HTTP/3: xk6 build --with github.com/bandorko/xk6-http3
// Then run with: ./k6 run http_benchmark.js -e PROTOCOL=http3
let http3;
try {
  http3 = require("k6/x/http3");
} catch (e) {
  // http3 extension not available, will use standard http
  http3 = null;
}

// --- Custom Metrics ---
const protocolLatency = new Trend("protocol_latency", true);
const smallPayloadDuration = new Trend("small_payload_duration", true);
const mediumPayloadDuration = new Trend("medium_payload_duration", true);
const largePayloadDuration = new Trend("large_payload_duration", true);
const successfulRequests = new Counter("successful_requests");

// HTTP/3 specific metrics
const http3Latency = new Trend("http3_latency", true);
const http2Latency = new Trend("http2_latency", true);
const http1Latency = new Trend("http1_latency", true);

// --- Environment Variables ---
// Set via command line:
// k6 run http_benchmark.js -e TARGET_HOST='https://http-evolution-benchmark.zjkung1123.workers.dev'
//
// For HTTP/3 testing:
// 1. Build k6 with HTTP/3: xk6 build --with github.com/bandorko/xk6-http3
// 2. Run with: ./k6 run http_benchmark.js -e PROTOCOL=http3
//
// PROTOCOL options: http1, http2, http3, compare (tests all protocols)
const TARGET_HOST =
  __ENV.TARGET_HOST ||
  "https://http-evolution-benchmark.zjkung1123.workers.dev";
const TEST_DURATION = __ENV.TEST_DURATION || "1m";
const PROTOCOL = __ENV.PROTOCOL || "http2"; // http1, http2, http3, compare

// Helper function to make requests based on protocol
function makeRequest(url, protocol) {
  if (protocol === "http3" && http3) {
    return http3.get(url);
  }
  // Standard k6 http uses HTTP/2 by default over TLS
  return http.get(url);
}

// --- Key configuration for k6 ---
export const options = {
  scenarios: {
    // Scenario 1: Constant load for baseline measurement
    constant_load: {
      executor: "constant-vus",
      vus: 10,
      duration: TEST_DURATION,
      tags: { scenario: "constant_load" },
    },
    // Scenario 2: Ramping load for stress testing
    ramping_load: {
      executor: "ramping-vus",
      startVUs: 1,
      stages: [
        { duration: "30s", target: 20 },
        { duration: "1m", target: 20 },
        { duration: "30s", target: 50 },
        { duration: "30s", target: 0 },
      ],
      startTime: TEST_DURATION, // Start after constant_load
      tags: { scenario: "ramping_load" },
    },
  },

  // Skip TLS verification if needed
  insecureSkipTLSVerify: true,

  // Custom thresholds
  thresholds: {
    http_req_duration: ["p(50)<200", "p(95)<500", "p(99)<1000"],
    checks: ["rate>0.99"],
    successful_requests: ["count>100"],
  },
};

// --- API Endpoints ---
const ENDPOINTS = {
  protocol: "/protocol",
  latency: "/latency",
  small: "/small",
  medium: "/medium",
  large: "/large",
  health: "/health",
};

// --- Main Test Function ---
export default function () {
  // Determine which protocol(s) to test
  const protocols =
    PROTOCOL === "compare"
      ? ["http2", ...(http3 ? ["http3"] : [])]
      : [PROTOCOL];

  for (const proto of protocols) {
    // Test 1: Protocol detection and latency baseline
    group(`[${proto.toUpperCase()}] Protocol & Latency Check`, function () {
      const protocolRes = makeRequest(
        `${TARGET_HOST}${ENDPOINTS.protocol}`,
        proto
      );

      check(protocolRes, {
        "protocol endpoint status 200": (r) => r.status === 200,
        "protocol response has protocol field": (r) => {
          try {
            const body = JSON.parse(r.body);
            return body.protocol !== undefined;
          } catch (e) {
            return false;
          }
        },
      });

      if (protocolRes.status === 200) {
        try {
          const body = JSON.parse(protocolRes.body);
          const duration = protocolRes.timings
            ? protocolRes.timings.duration
            : 0;

          protocolLatency.add(duration, {
            protocol: body.protocol,
            colo: body.colo || "unknown",
          });

          // Add to protocol-specific metric
          if (body.protocol === "HTTP/3") {
            http3Latency.add(duration);
          } else if (body.protocol === "HTTP/2") {
            http2Latency.add(duration);
          } else {
            http1Latency.add(duration);
          }

          successfulRequests.add(1);
        } catch (e) {
          // JSON parse error
        }
      }
    });

    // Test 2: Latency endpoint (minimal payload)
    group(`[${proto.toUpperCase()}] Latency Test`, function () {
      const latencyRes = makeRequest(
        `${TARGET_HOST}${ENDPOINTS.latency}`,
        proto
      );
      const duration = latencyRes.timings ? latencyRes.timings.duration : 0;

      check(latencyRes, {
        "latency endpoint status 200": (r) => r.status === 200,
        "latency response time < 100ms": (r) => {
          const d = r.timings ? r.timings.duration : 0;
          return d < 100;
        },
      });

      if (latencyRes.status === 200) {
        successfulRequests.add(1);
      }
    });

    // Test 3: Small payload (~100 bytes)
    group(`[${proto.toUpperCase()}] Small Payload Test`, function () {
      const smallRes = makeRequest(`${TARGET_HOST}${ENDPOINTS.small}`, proto);

      check(smallRes, {
        "small payload status 200": (r) => r.status === 200,
        "small payload has data": (r) => {
          try {
            const body = JSON.parse(r.body);
            return body.size === "small";
          } catch (e) {
            return false;
          }
        },
      });

      if (smallRes.status === 200) {
        const duration = smallRes.timings ? smallRes.timings.duration : 0;
        smallPayloadDuration.add(duration, { protocol: proto });
        successfulRequests.add(1);
      }
    });

    // Test 4: Medium payload (~10KB)
    group(`[${proto.toUpperCase()}] Medium Payload Test`, function () {
      const mediumRes = makeRequest(`${TARGET_HOST}${ENDPOINTS.medium}`, proto);

      check(mediumRes, {
        "medium payload status 200": (r) => r.status === 200,
        "medium payload has items": (r) => {
          try {
            const body = JSON.parse(r.body);
            return body.items && body.items.length === 100;
          } catch (e) {
            return false;
          }
        },
      });

      if (mediumRes.status === 200) {
        const duration = mediumRes.timings ? mediumRes.timings.duration : 0;
        mediumPayloadDuration.add(duration, { protocol: proto });
        successfulRequests.add(1);
      }
    });

    // Test 5: Large payload (~100KB)
    group(`[${proto.toUpperCase()}] Large Payload Test`, function () {
      const largeRes = makeRequest(`${TARGET_HOST}${ENDPOINTS.large}`, proto);

      check(largeRes, {
        "large payload status 200": (r) => r.status === 200,
        "large payload has items": (r) => {
          try {
            const body = JSON.parse(r.body);
            return body.items && body.items.length === 500;
          } catch (e) {
            return false;
          }
        },
      });

      if (largeRes.status === 200) {
        const duration = largeRes.timings ? largeRes.timings.duration : 0;
        largePayloadDuration.add(duration, { protocol: proto });
        successfulRequests.add(1);
      }
    });
  } // End of protocols loop

  // Test 6: Batch requests (HTTP/2 only - batch not supported in http3 extension)
  if (PROTOCOL !== "http3") {
    group("Batch Request Test (HTTP/2)", function () {
      const batchRequests = [
        ["GET", `${TARGET_HOST}${ENDPOINTS.protocol}`],
        ["GET", `${TARGET_HOST}${ENDPOINTS.small}`],
        ["GET", `${TARGET_HOST}${ENDPOINTS.medium}`],
      ];

      const responses = http.batch(batchRequests);

      responses.forEach((res, index) => {
        check(res, {
          [`Batch request ${index + 1} status 200`]: (r) => r.status === 200,
        });
        if (res.status === 200) {
          successfulRequests.add(1);
        }
      });
    });
  }

  // Small pause between iterations
  sleep(0.5);
}

// --- Setup Function ---
export function setup() {
  console.log(`\n========================================`);
  console.log(`HTTP Evolution Benchmark`);
  console.log(`========================================`);
  console.log(`Target: ${TARGET_HOST}`);
  console.log(`Duration: ${TEST_DURATION}`);
  console.log(`Protocol Mode: ${PROTOCOL}`);
  console.log(`HTTP/3 Extension: ${http3 ? "Available" : "Not Available"}`);
  console.log(`========================================\n`);

  if (PROTOCOL === "http3" && !http3) {
    console.log(
      `WARNING: HTTP/3 requested but xk6-http3 extension not available!`
    );
    console.log(
      `Build k6 with: xk6 build --with github.com/bandorko/xk6-http3`
    );
    console.log(`Falling back to HTTP/2...\n`);
  }

  // Verify connectivity
  const healthCheck = http.get(`${TARGET_HOST}${ENDPOINTS.health}`);
  if (healthCheck.status !== 200) {
    throw new Error(`Health check failed! Status: ${healthCheck.status}`);
  }

  // Get initial protocol info
  const protocolCheck = http.get(`${TARGET_HOST}${ENDPOINTS.protocol}`);
  if (protocolCheck.status === 200) {
    const info = JSON.parse(protocolCheck.body);
    console.log(`Server Protocol: ${info.protocol}`);
    console.log(`TLS Version: ${info.tls_version}`);
    console.log(`Cloudflare Colo: ${info.colo}`);
    console.log(`Country: ${info.country}`);
  }

  return { startTime: Date.now() };
}

// --- Teardown Function ---
export function teardown(data) {
  const duration = (Date.now() - data.startTime) / 1000;
  console.log(`\n========================================`);
  console.log(`Benchmark Complete!`);
  console.log(`Total Duration: ${duration.toFixed(2)}s`);
  console.log(`========================================\n`);
}
