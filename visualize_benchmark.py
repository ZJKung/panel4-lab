#!/usr/bin/env -S uv run
# /// script
# requires-python = ">=3.10"
# dependencies = [
#     "matplotlib>=3.8.0",
#     "numpy>=1.26.0",
# ]
# ///
"""
HTTP Protocol Benchmark Visualization
Compares HTTP/1.1, HTTP/2, and HTTP/3 performance metrics

Usage:
    uv run visualize_benchmark.py [results_file.json]

    If no file is specified, uses results/latest.json
"""

import json
import sys
import os
import matplotlib.pyplot as plt
import numpy as np

# Get the result file from command line or use default
if len(sys.argv) > 1:
    result_file = sys.argv[1]
else:
    # Try results/latest.json first, then fall back to benchmark_results.json
    if os.path.exists("results/latest.json"):
        result_file = "results/latest.json"
    else:
        result_file = "benchmark_results.json"

print(f"Loading results from: {result_file}")

# Load benchmark results
with open(result_file, "r") as f:
    data = json.load(f)

results = data["results"]

# Set up the figure with subplots
fig, axes = plt.subplots(2, 2, figsize=(14, 10))
fig.suptitle(
    "HTTP Protocol Performance Comparison\n(HTTP/1.1 vs HTTP/2 vs HTTP/3)",
    fontsize=14,
    fontweight="bold",
)

# Color palette
colors = {"http1_1": "#E74C3C", "http2": "#3498DB", "http3": "#2ECC71"}
labels = {"http1_1": "HTTP/1.1", "http2": "HTTP/2", "http3": "HTTP/3"}

# 1. Single Request Latency
ax1 = axes[0, 0]
protocols = ["http1_1", "http2", "http3"]
x = np.arange(len(protocols))
width = 0.35

total_times = [results["single_request"][p]["avg_total"] * 1000 for p in protocols]
ttfb_times = [results["single_request"][p]["avg_ttfb"] * 1000 for p in protocols]

bars1 = ax1.bar(
    x - width / 2,
    total_times,
    width,
    label="Total Time",
    color=[colors[p] for p in protocols],
    alpha=0.8,
)
bars2 = ax1.bar(
    x + width / 2,
    ttfb_times,
    width,
    label="TTFB",
    color=[colors[p] for p in protocols],
    alpha=0.5,
    hatch="//",
)

ax1.set_xlabel("Protocol")
ax1.set_ylabel("Time (ms)")
ax1.set_title("Single Request Latency\n(API Endpoint)")
ax1.set_xticks(x)
ax1.set_xticklabels([labels[p] for p in protocols])
ax1.legend()
ax1.grid(axis="y", alpha=0.3)

# Add value labels
for bar, val in zip(bars1, total_times):
    ax1.text(
        bar.get_x() + bar.get_width() / 2,
        bar.get_height() + 1,
        f"{val:.1f}",
        ha="center",
        va="bottom",
        fontsize=9,
    )

# 2. Large File Download
ax2 = axes[0, 1]

total_times_large = [results["large_file"][p]["avg_total"] * 1000 for p in protocols]
ttfb_times_large = [results["large_file"][p]["avg_ttfb"] * 1000 for p in protocols]

bars1 = ax2.bar(
    x - width / 2,
    total_times_large,
    width,
    label="Total Time",
    color=[colors[p] for p in protocols],
    alpha=0.8,
)
bars2 = ax2.bar(
    x + width / 2,
    ttfb_times_large,
    width,
    label="TTFB",
    color=[colors[p] for p in protocols],
    alpha=0.5,
    hatch="//",
)

ax2.set_xlabel("Protocol")
ax2.set_ylabel("Time (ms)")
ax2.set_title("Large File Download (~1MB)\n(Throughput Test)")
ax2.set_xticks(x)
ax2.set_xticklabels([labels[p] for p in protocols])
ax2.legend()
ax2.grid(axis="y", alpha=0.3)

# Add value labels
for bar, val in zip(bars1, total_times_large):
    ax2.text(
        bar.get_x() + bar.get_width() / 2,
        bar.get_height() + 3,
        f"{val:.0f}",
        ha="center",
        va="bottom",
        fontsize=9,
    )

# 3. Concurrent Downloads (Multiplexing)
ax3 = axes[1, 0]

concurrent_times = [results["concurrent"][p]["total_time"] * 1000 for p in protocols]

bars = ax3.bar(
    x,
    concurrent_times,
    color=[colors[p] for p in protocols],
    alpha=0.8,
    edgecolor="black",
    linewidth=1,
)

ax3.set_xlabel("Protocol")
ax3.set_ylabel("Time (ms)")
ax3.set_title("10 Concurrent Image Downloads\n(Multiplexing Test)")
ax3.set_xticks(x)
ax3.set_xticklabels([labels[p] for p in protocols])
ax3.grid(axis="y", alpha=0.3)

# Add value labels and improvement percentages
baseline = concurrent_times[0]  # HTTP/1.1
for bar, val, p in zip(bars, concurrent_times, protocols):
    improvement = ((baseline - val) / baseline) * 100
    ax3.text(
        bar.get_x() + bar.get_width() / 2,
        bar.get_height() + 5,
        f"{val:.0f}ms",
        ha="center",
        va="bottom",
        fontsize=10,
        fontweight="bold",
    )
    if p != "http1_1" and improvement > 5:
        ax3.text(
            bar.get_x() + bar.get_width() / 2,
            bar.get_height() / 2,
            f"{improvement:.0f}% faster",
            ha="center",
            va="center",
            fontsize=9,
            color="white",
            fontweight="bold",
        )

# 4. Full Page Load
ax4 = axes[1, 1]

page_load_times = [results["full_page"][p]["total_time"] * 1000 for p in protocols]

bars = ax4.bar(
    x,
    page_load_times,
    color=[colors[p] for p in protocols],
    alpha=0.8,
    edgecolor="black",
    linewidth=1,
)

ax4.set_xlabel("Protocol")
ax4.set_ylabel("Time (ms)")
ax4.set_title("Full Page Load (HTML + 20 images)\n(Real-world Simulation)")
ax4.set_xticks(x)
ax4.set_xticklabels([labels[p] for p in protocols])
ax4.grid(axis="y", alpha=0.3)

# Add value labels
baseline = page_load_times[0]  # HTTP/1.1
for bar, val, p in zip(bars, page_load_times, protocols):
    improvement = ((baseline - val) / baseline) * 100
    ax4.text(
        bar.get_x() + bar.get_width() / 2,
        bar.get_height() + 1,
        f"{val:.0f}ms",
        ha="center",
        va="bottom",
        fontsize=10,
        fontweight="bold",
    )
    if p != "http1_1" and improvement > 5:
        ax4.text(
            bar.get_x() + bar.get_width() / 2,
            bar.get_height() / 2,
            f"{improvement:.0f}% faster",
            ha="center",
            va="center",
            fontsize=9,
            color="white",
            fontweight="bold",
        )

plt.tight_layout()

# Save outputs
output_base = os.path.splitext(result_file)[0]
png_file = f"{output_base}_chart.png"
svg_file = f"{output_base}_chart.svg"

plt.savefig(png_file, dpi=150, bbox_inches="tight")
plt.savefig(svg_file, bbox_inches="tight")
print(f"Saved: {png_file}")
print(f"Saved: {svg_file}")

# Create a summary table
print("\n" + "=" * 60)
print("HTTP PROTOCOL BENCHMARK SUMMARY")
print("=" * 60)
print(f"Target: {data['target']}")
print(f"Date: {data['benchmark_date']}")
print(f"Iterations: {data['iterations']}")
print("=" * 60)

print("\n📊 Single Request Latency (API Endpoint)")
print("-" * 40)
for p in protocols:
    avg = results["single_request"][p]["avg_total"] * 1000
    print(f"  {labels[p]:10s}: {avg:6.1f} ms")

print("\n📦 Large File Download (~1MB)")
print("-" * 40)
for p in protocols:
    avg = results["large_file"][p]["avg_total"] * 1000
    ttfb = results["large_file"][p]["avg_ttfb"] * 1000
    print(f"  {labels[p]:10s}: {avg:6.1f} ms (TTFB: {ttfb:.1f} ms)")

print("\n🔀 Concurrent Downloads (10 images)")
print("-" * 40)
for p in protocols:
    t = results["concurrent"][p]["total_time"] * 1000
    speedup = (
        results["concurrent"]["http1_1"]["total_time"]
        / results["concurrent"][p]["total_time"]
    )
    print(f"  {labels[p]:10s}: {t:6.1f} ms ({speedup:.2f}x vs HTTP/1.1)")

print("\n🌐 Full Page Load (HTML + 20 images)")
print("-" * 40)
for p in protocols:
    t = results["full_page"][p]["total_time"] * 1000
    speedup = (
        results["full_page"]["http1_1"]["total_time"]
        / results["full_page"][p]["total_time"]
    )
    print(f"  {labels[p]:10s}: {t:6.1f} ms ({speedup:.2f}x vs HTTP/1.1)")

# Determine winners
print("\n" + "=" * 60)
print("🏆 WINNERS BY CATEGORY:")
print("=" * 60)

# Single request winner
single_times = {p: results["single_request"][p]["avg_total"] for p in protocols}
single_winner = min(single_times, key=single_times.get)
print(f"  Single Request Latency: {labels[single_winner]} ✅")

# Large file winner
large_times = {p: results["large_file"][p]["avg_total"] for p in protocols}
large_winner = min(large_times, key=large_times.get)
print(f"  Large File Download:    {labels[large_winner]} ✅")

# Concurrent winner
concurrent_results = {p: results["concurrent"][p]["total_time"] for p in protocols}
concurrent_winner = min(concurrent_results, key=concurrent_results.get)
print(f"  Concurrent Downloads:   {labels[concurrent_winner]} ✅")

# Full page winner
page_times = {p: results["full_page"][p]["total_time"] for p in protocols}
page_winner = min(page_times, key=page_times.get)
print(f"  Full Page Load:         {labels[page_winner]} ✅")

print("=" * 60)

plt.show()
