import http from "k6/http";
import { check, group, sleep } from "k6";

// --- Environment Variables ---
// Set via command line:
// k6 run http_benchmark.js -e TARGET_HOST='https://your-server-ip' -e K6_PROTOCOL='3'
const TARGET_HOST = __ENV.TARGET_HOST || "https://localhost";
const PROTOCOL = __ENV.K6_PROTOCOL || "1.1"; // The protocol being tested
const NUM_LARGE_IMAGES = 5; // Images 1-5 are large
const NUM_SMALL_IMAGES = 95; // Images 6-100 are small

// --- Key configuration for k6 ---
export const options = {
  // Ramp up and maintain VUs for stress testing (adjust as needed)
  stages: [
    { duration: "30s", target: 5 },
    { duration: "1m", target: 5 },
    { duration: "30s", target: 0 },
  ],
  // Skip TLS verification if using self-signed certificates
  insecureSkipTLSVerify: true,
  // Define a custom tag to easily filter results by protocol in the output
  tags: {
    protocol_version: `HTTP/${PROTOCOL}`,
    target_host: TARGET_HOST,
  },
  // Custom thresholds focused on speed and success rate
  thresholds: {
    http_req_duration: ["p(95)<5000", "p(99)<7000"], // 95% of requests must finish under 5s
    checks: ["rate>0.99"], // 99% of requests must pass the checks
  },
  // Ensure k6 attempts to use the specified protocol
  ext: {
    http: {
      // Set the protocol to use (k6 will negotiate h1/h2/h3 based on this)
      protocol: `http${PROTOCOL}`,
    },
  },
};

// --- Array of all resources to load ---
function buildAssetList() {
  const assets = ["/index.html", "/style.css", "/app.js"];
  // Add large images (1-5)
  for (let i = 1; i <= NUM_LARGE_IMAGES; i++) {
    assets.push(
      `/images/image_${String(i).padStart(3, "0")}_large_1200x1200.jpg`
    );
  }
  // Add small images (6-100)
  for (let i = 1; i <= NUM_SMALL_IMAGES; i++) {
    const imageNum = NUM_LARGE_IMAGES + i;
    assets.push(
      `/images/image_${String(imageNum).padStart(3, "0")}_small_300x300.jpg`
    );
  }
  return assets;
}

const ASSETS = buildAssetList();

export default function () {
  group(`Load Test Page on HTTP/${PROTOCOL}`, function () {
    // Construct the full array of requests for http.batch
    const requests = ASSETS.map((asset) => ["GET", TARGET_HOST + asset]);

    // http.batch sends all requests concurrently, simulating a browser
    const responses = http.batch(requests);

    // Check the success of all requests
    responses.forEach((res, index) => {
      check(res, {
        [`Asset ${ASSETS[index]} loaded (Status 200)`]: (r) => r.status === 200,
      });
      // Check that the negotiated protocol matches the target protocol tag
      check(res, {
        [`Protocol used is H${PROTOCOL}`]: (r) => r.proto.includes(PROTOCOL),
      });
    });
  });

  // Pause between iterations to simulate user "think time"
  sleep(1);
}
