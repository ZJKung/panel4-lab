/**
 * HTTP Evolution Benchmark - Cloudflare Workers
 *
 * This worker provides endpoints for benchmarking HTTP/1.1, HTTP/2, and HTTP/3 performance.
 * Cloudflare automatically handles HTTP/3 (QUIC) at the edge.
 */

export default {
  async fetch(request, env, ctx) {
    const url = new URL(request.url);
    const path = url.pathname;

    // Add Cache-Control: no-store to all responses for accurate benchmarking
    const headers = {
      "Cache-Control": "no-store, no-cache, must-revalidate",
      "Content-Type": "application/json",
    };

    // API Route handling
    switch (path) {
      case "/api":
        return new Response(
          JSON.stringify(
            {
              message: "HTTP Evolution Benchmark Server (Cloudflare Workers)",
              endpoints: {
                "/": "Static HTML page with images for benchmark testing",
                "/api": "This API help message",
                "/api/health": "Health check endpoint",
                "/api/protocol":
                  "Show protocol information (HTTP/1.1, HTTP/2, HTTP/3)",
                "/100mb.bin":
                  "100MB binary file for large file download benchmark (dynamically generated)",
              },
              note: "Cloudflare automatically serves HTTP/3 when client supports it",
            },
            null,
            2
          ),
          { headers }
        );

      case "/api/health":
        return new Response(JSON.stringify({ status: "ok" }), { headers });

      case "/api/protocol":
        return handleProtocol(request, headers);

      case "/100mb.bin":
        return handleLargeFile(100);

      default:
        // For non-API routes, let Cloudflare's static asset serving handle it
        // This will serve files from the static folder (index.html, images, etc.)
        return env.ASSETS.fetch(request);
    }
  },
};

/**
 * Protocol endpoint - shows which HTTP version the client is using
 *
 * Cloudflare provides the cf object with connection info including HTTP version
 */
function handleProtocol(request, headers) {
  // cf.httpProtocol gives us the actual protocol used by the client
  // This is the real HTTP version (HTTP/1.1, HTTP/2, or HTTP/3)
  const cf = request.cf || {};

  const response = {
    // The actual HTTP protocol used between client and Cloudflare edge
    protocol: cf.httpProtocol || "unknown",

    // Additional connection info
    tls_version: cf.tlsVersion || "unknown",
    tls_cipher: cf.tlsCipher || "unknown",

    // Client location info (useful for latency analysis)
    colo: cf.colo || "unknown", // Cloudflare datacenter code (e.g., SIN, IAD)
    country: cf.country || "unknown",
    city: cf.city || "unknown",

    // Request metadata
    client_ip: request.headers.get("CF-Connecting-IP") || "unknown",
    ray_id: request.headers.get("CF-Ray") || "unknown",
  };

  return new Response(JSON.stringify(response, null, 2), { headers });
}

/**
 * Large file endpoint - generates a binary file of specified size for download benchmarking
 *
 * Uses streaming to efficiently generate large files without memory issues
 * @param {number} sizeMB - Size of the file in megabytes
 */
function handleLargeFile(sizeMB) {
  const sizeBytes = sizeMB * 1024 * 1024;
  const chunkSize = 64 * 1024; // 64KB chunks for efficient streaming

  // Create a ReadableStream that generates binary data
  const stream = new ReadableStream({
    start(controller) {
      let bytesRemaining = sizeBytes;

      function pushChunk() {
        if (bytesRemaining <= 0) {
          controller.close();
          return;
        }

        const currentChunkSize = Math.min(chunkSize, bytesRemaining);
        const chunk = new Uint8Array(currentChunkSize);

        // Fill with pseudo-random data for more realistic download testing
        for (let i = 0; i < currentChunkSize; i++) {
          chunk[i] = (i * 7 + bytesRemaining) & 0xff;
        }

        controller.enqueue(chunk);
        bytesRemaining -= currentChunkSize;

        // Continue pushing chunks
        pushChunk();
      }

      pushChunk();
    },
  });

  return new Response(stream, {
    headers: {
      "Content-Type": "application/octet-stream",
      "Content-Length": sizeBytes.toString(),
      "Content-Disposition": `attachment; filename="${sizeMB}mb.bin"`,
      "Cache-Control": "no-store, no-cache, must-revalidate",
    },
  });
}
