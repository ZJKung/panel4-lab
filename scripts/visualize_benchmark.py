#!/usr/bin/env python3
"""
HTTP Protocol Benchmark Visualization Tool

This script reads benchmark JSON results and creates visualizations
comparing HTTP/1.1, HTTP/2, and HTTP/3 performance metrics.

Usage:
    python visualize_benchmark.py <results_folder>
    python visualize_benchmark.py results/benchmark_2024-12-02_15-30-00.json
"""

import json
import os
import sys
from pathlib import Path
from datetime import datetime

try:
    import matplotlib.pyplot as plt
    import matplotlib.patches as mpatches
    import numpy as np
except ImportError:
    print("Error: matplotlib and numpy are required.")
    print("Install with: pip install matplotlib numpy")
    sys.exit(1)


# Color scheme for protocols
COLORS = {
    "h1": "#FF6B6B",  # Red for HTTP/1.1
    "h2": "#4ECDC4",  # Teal for HTTP/2
    "h3": "#45B7D1",  # Blue for HTTP/3
}

PROTOCOL_NAMES = {
    "h1": "HTTP/1.1",
    "h2": "HTTP/2",
    "h3": "HTTP/3 (QUIC)",
}


def load_results(path: str) -> dict:
    """Load benchmark results from JSON file or find latest in folder."""
    path = Path(path)

    if path.is_file():
        with open(path, "r") as f:
            return json.load(f)

    if path.is_dir():
        # Find all JSON files
        json_files = sorted(path.glob("benchmark_*.json"), reverse=True)
        if not json_files:
            raise FileNotFoundError(f"No benchmark JSON files found in {path}")

        latest = json_files[0]
        print(f"Loading latest results: {latest}")
        with open(latest, "r") as f:
            return json.load(f)

    raise FileNotFoundError(f"Path not found: {path}")


def create_timing_breakdown_chart(results: dict, output_dir: Path):
    """Create a stacked bar chart showing timing breakdown for each protocol."""
    fig, ax = plt.subplots(figsize=(12, 6))

    protocols = []
    dns_times = []
    tcp_times = []
    tls_times = []
    ttfb_times = []
    transfer_times = []

    for proto in ["h1", "h2", "h3"]:
        if proto in results:
            protocols.append(PROTOCOL_NAMES[proto])
            r = results[proto]
            dns_times.append(r.get("avg_dns_lookup_ms", 0))
            tcp_times.append(r.get("avg_tcp_connect_ms", 0))
            tls_times.append(r.get("avg_tls_handshake_ms", 0))
            # TTFB minus connection overhead
            ttfb_overhead = (
                r.get("avg_ttfb_ms", 0)
                - r.get("avg_dns_lookup_ms", 0)
                - r.get("avg_tcp_connect_ms", 0)
                - r.get("avg_tls_handshake_ms", 0)
            )
            ttfb_times.append(max(0, ttfb_overhead))
            transfer_times.append(r.get("avg_content_transfer_ms", 0))

    x = np.arange(len(protocols))
    width = 0.5

    # Create stacked bars
    bars1 = ax.bar(x, dns_times, width, label="DNS Lookup", color="#FFE66D")
    bars2 = ax.bar(
        x, tcp_times, width, bottom=dns_times, label="TCP Connect", color="#FF6B6B"
    )

    bottom2 = [d + t for d, t in zip(dns_times, tcp_times)]
    bars3 = ax.bar(
        x, tls_times, width, bottom=bottom2, label="TLS Handshake", color="#4ECDC4"
    )

    bottom3 = [b + t for b, t in zip(bottom2, tls_times)]
    bars4 = ax.bar(
        x, ttfb_times, width, bottom=bottom3, label="Server Processing", color="#45B7D1"
    )

    bottom4 = [b + t for b, t in zip(bottom3, ttfb_times)]
    bars5 = ax.bar(
        x,
        transfer_times,
        width,
        bottom=bottom4,
        label="Content Transfer",
        color="#96CEB4",
    )

    ax.set_ylabel("Time (ms)", fontsize=12)
    ax.set_title("Request Timing Breakdown by Protocol", fontsize=14, fontweight="bold")
    ax.set_xticks(x)
    ax.set_xticklabels(protocols, fontsize=11)
    ax.legend(loc="upper right")
    ax.grid(axis="y", alpha=0.3)

    # Add total time labels on top of bars
    totals = [
        sum(vals)
        for vals in zip(dns_times, tcp_times, tls_times, ttfb_times, transfer_times)
    ]
    for i, total in enumerate(totals):
        ax.annotate(
            f"{total:.1f}ms",
            xy=(i, total),
            ha="center",
            va="bottom",
            fontsize=10,
            fontweight="bold",
        )

    plt.tight_layout()
    output_path = output_dir / "timing_breakdown.png"
    plt.savefig(output_path, dpi=150, bbox_inches="tight")
    plt.close()
    print(f"  ✅ Saved: {output_path}")


def create_latency_comparison_chart(results: dict, output_dir: Path):
    """Create a grouped bar chart comparing latency percentiles."""
    fig, ax = plt.subplots(figsize=(12, 6))

    protocols = []
    p50_times = []
    p95_times = []
    p99_times = []
    avg_times = []

    for proto in ["h1", "h2", "h3"]:
        if proto in results:
            protocols.append(PROTOCOL_NAMES[proto])
            r = results[proto]
            p50_times.append(r.get("p50_total_time_ms", 0))
            p95_times.append(r.get("p95_total_time_ms", 0))
            p99_times.append(r.get("p99_total_time_ms", 0))
            avg_times.append(r.get("avg_total_time_ms", 0))

    x = np.arange(len(protocols))
    width = 0.2

    bars1 = ax.bar(x - 1.5 * width, avg_times, width, label="Average", color="#45B7D1")
    bars2 = ax.bar(
        x - 0.5 * width, p50_times, width, label="P50 (Median)", color="#4ECDC4"
    )
    bars3 = ax.bar(x + 0.5 * width, p95_times, width, label="P95", color="#FFE66D")
    bars4 = ax.bar(x + 1.5 * width, p99_times, width, label="P99", color="#FF6B6B")

    ax.set_ylabel("Latency (ms)", fontsize=12)
    ax.set_title("Latency Distribution by Protocol", fontsize=14, fontweight="bold")
    ax.set_xticks(x)
    ax.set_xticklabels(protocols, fontsize=11)
    ax.legend(loc="upper right")
    ax.grid(axis="y", alpha=0.3)

    # Add value labels on bars
    def add_labels(bars):
        for bar in bars:
            height = bar.get_height()
            ax.annotate(
                f"{height:.1f}",
                xy=(bar.get_x() + bar.get_width() / 2, height),
                ha="center",
                va="bottom",
                fontsize=8,
            )

    add_labels(bars1)
    add_labels(bars2)
    add_labels(bars3)
    add_labels(bars4)

    plt.tight_layout()
    output_path = output_dir / "latency_comparison.png"
    plt.savefig(output_path, dpi=150, bbox_inches="tight")
    plt.close()
    print(f"  ✅ Saved: {output_path}")


def create_throughput_chart(results: dict, output_dir: Path):
    """Create a bar chart comparing throughput (requests per second)."""
    fig, ax = plt.subplots(figsize=(10, 6))

    protocols = []
    throughputs = []
    colors = []

    for proto in ["h1", "h2", "h3"]:
        if proto in results:
            protocols.append(PROTOCOL_NAMES[proto])
            throughputs.append(results[proto].get("requests_per_second", 0))
            colors.append(COLORS[proto])

    x = np.arange(len(protocols))
    bars = ax.bar(x, throughputs, color=colors, edgecolor="black", linewidth=1.2)

    ax.set_ylabel("Requests per Second", fontsize=12)
    ax.set_title("Throughput Comparison", fontsize=14, fontweight="bold")
    ax.set_xticks(x)
    ax.set_xticklabels(protocols, fontsize=11)
    ax.grid(axis="y", alpha=0.3)

    # Add value labels on bars
    for bar in bars:
        height = bar.get_height()
        ax.annotate(
            f"{height:.1f} req/s",
            xy=(bar.get_x() + bar.get_width() / 2, height),
            ha="center",
            va="bottom",
            fontsize=11,
            fontweight="bold",
        )

    # Add improvement percentages
    if len(throughputs) >= 2:
        baseline = throughputs[0]  # HTTP/1.1 as baseline
        for i, (bar, tp) in enumerate(zip(bars[1:], throughputs[1:]), 1):
            improvement = ((tp - baseline) / baseline) * 100
            ax.annotate(
                f"+{improvement:.0f}%",
                xy=(bar.get_x() + bar.get_width() / 2, bar.get_height() / 2),
                ha="center",
                va="center",
                fontsize=10,
                color="white",
                fontweight="bold",
            )

    plt.tight_layout()
    output_path = output_dir / "throughput_comparison.png"
    plt.savefig(output_path, dpi=150, bbox_inches="tight")
    plt.close()
    print(f"  ✅ Saved: {output_path}")


def create_min_max_chart(results: dict, output_dir: Path):
    """Create a chart showing min/max/avg response times with error bars."""
    fig, ax = plt.subplots(figsize=(10, 6))

    protocols = []
    avgs = []
    mins = []
    maxs = []
    colors = []

    for proto in ["h1", "h2", "h3"]:
        if proto in results:
            protocols.append(PROTOCOL_NAMES[proto])
            r = results[proto]
            avg = r.get("avg_total_time_ms", 0)
            min_val = r.get("min_total_time_ms", 0)
            max_val = r.get("max_total_time_ms", 0)
            avgs.append(avg)
            mins.append(avg - min_val)
            maxs.append(max_val - avg)
            colors.append(COLORS[proto])

    x = np.arange(len(protocols))

    # Plot bars with error bars
    bars = ax.bar(x, avgs, color=colors, edgecolor="black", linewidth=1.2, alpha=0.8)
    ax.errorbar(
        x,
        avgs,
        yerr=[mins, maxs],
        fmt="none",
        ecolor="black",
        capsize=8,
        capthick=2,
        elinewidth=2,
    )

    ax.set_ylabel("Response Time (ms)", fontsize=12)
    ax.set_title(
        "Response Time Range (Min / Avg / Max)", fontsize=14, fontweight="bold"
    )
    ax.set_xticks(x)
    ax.set_xticklabels(protocols, fontsize=11)
    ax.grid(axis="y", alpha=0.3)

    # Add annotations
    for i, (proto, avg, min_v, max_v) in enumerate(zip(protocols, avgs, mins, maxs)):
        ax.annotate(
            f"Avg: {avg:.1f}ms\nMin: {avg-min_v:.1f}ms\nMax: {avg+max_v:.1f}ms",
            xy=(i, avg + max_v + 5),
            ha="center",
            va="bottom",
            fontsize=9,
        )

    plt.tight_layout()
    output_path = output_dir / "response_time_range.png"
    plt.savefig(output_path, dpi=150, bbox_inches="tight")
    plt.close()
    print(f"  ✅ Saved: {output_path}")


def create_summary_dashboard(results: dict, metadata: dict, output_dir: Path):
    """Create a comprehensive dashboard with all metrics."""
    fig = plt.figure(figsize=(16, 12))

    # Title
    fig.suptitle(
        "HTTP Protocol Benchmark Results", fontsize=18, fontweight="bold", y=0.98
    )

    # Metadata subtitle
    url = metadata.get("url", "N/A")
    timestamp = metadata.get("timestamp", "N/A")
    requests = metadata.get("requests", "N/A")
    concurrency = metadata.get("concurrency", "N/A")
    fig.text(
        0.5,
        0.94,
        f"URL: {url}\nRequests: {requests} | Concurrency: {concurrency} | Time: {timestamp}",
        ha="center",
        fontsize=10,
        style="italic",
    )

    # Create 2x2 grid of subplots
    gs = fig.add_gridspec(2, 2, hspace=0.3, wspace=0.3, top=0.88, bottom=0.08)

    # 1. Timing Breakdown (top-left)
    ax1 = fig.add_subplot(gs[0, 0])
    protocols = []
    totals = []
    colors_list = []

    for proto in ["h1", "h2", "h3"]:
        if proto in results:
            protocols.append(PROTOCOL_NAMES[proto])
            totals.append(results[proto].get("avg_total_time_ms", 0))
            colors_list.append(COLORS[proto])

    bars = ax1.bar(protocols, totals, color=colors_list, edgecolor="black")
    ax1.set_ylabel("Time (ms)")
    ax1.set_title("Average Response Time", fontweight="bold")
    ax1.grid(axis="y", alpha=0.3)
    for bar in bars:
        ax1.annotate(
            f"{bar.get_height():.1f}ms",
            xy=(bar.get_x() + bar.get_width() / 2, bar.get_height()),
            ha="center",
            va="bottom",
            fontweight="bold",
        )

    # 2. Throughput (top-right)
    ax2 = fig.add_subplot(gs[0, 1])
    throughputs = []
    for proto in ["h1", "h2", "h3"]:
        if proto in results:
            throughputs.append(results[proto].get("requests_per_second", 0))

    bars = ax2.bar(protocols, throughputs, color=colors_list, edgecolor="black")
    ax2.set_ylabel("Requests/sec")
    ax2.set_title("Throughput", fontweight="bold")
    ax2.grid(axis="y", alpha=0.3)
    for bar in bars:
        ax2.annotate(
            f"{bar.get_height():.1f}",
            xy=(bar.get_x() + bar.get_width() / 2, bar.get_height()),
            ha="center",
            va="bottom",
            fontweight="bold",
        )

    # 3. Latency Percentiles (bottom-left)
    ax3 = fig.add_subplot(gs[1, 0])
    x = np.arange(len(protocols))
    width = 0.25

    p50s = [
        results[p].get("p50_total_time_ms", 0)
        for p in ["h1", "h2", "h3"]
        if p in results
    ]
    p95s = [
        results[p].get("p95_total_time_ms", 0)
        for p in ["h1", "h2", "h3"]
        if p in results
    ]
    p99s = [
        results[p].get("p99_total_time_ms", 0)
        for p in ["h1", "h2", "h3"]
        if p in results
    ]

    ax3.bar(x - width, p50s, width, label="P50", color="#4ECDC4")
    ax3.bar(x, p95s, width, label="P95", color="#FFE66D")
    ax3.bar(x + width, p99s, width, label="P99", color="#FF6B6B")
    ax3.set_ylabel("Latency (ms)")
    ax3.set_title("Latency Percentiles", fontweight="bold")
    ax3.set_xticks(x)
    ax3.set_xticklabels(protocols)
    ax3.legend()
    ax3.grid(axis="y", alpha=0.3)

    # 4. Summary Stats as Text (bottom-right)
    ax4 = fig.add_subplot(gs[1, 1])
    ax4.axis("off")

    # Build summary text
    summary_text = "SUMMARY STATISTICS\n" + "=" * 40 + "\n\n"
    summary_text += f"{'Metric':<20} {'H1':>8} {'H2':>8} {'H3':>8}\n"
    summary_text += "-" * 46 + "\n"

    metrics = [
        ("Avg Time (ms)", "avg_total_time_ms"),
        ("TTFB (ms)", "avg_ttfb_ms"),
        ("TLS (ms)", "avg_tls_handshake_ms"),
        ("P99 (ms)", "p99_total_time_ms"),
        ("Throughput", "requests_per_second"),
    ]

    for metric_name, key in metrics:
        vals = []
        for proto in ["h1", "h2", "h3"]:
            if proto in results:
                val = results[proto].get(key, 0)
                vals.append(f"{val:.1f}")
            else:
                vals.append("N/A")
        summary_text += f"{metric_name:<20} {vals[0]:>8} {vals[1]:>8} {vals[2]:>8}\n"

    # Add success rates
    summary_text += "-" * 46 + "\n"
    vals = []
    for proto in ["h1", "h2", "h3"]:
        if proto in results:
            total = results[proto].get("total_requests", 1)
            success = results[proto].get("successful_requests", 0)
            vals.append(f"{(success/total)*100:.0f}%")
        else:
            vals.append("N/A")
    summary_text += f"{'Success Rate':<20} {vals[0]:>8} {vals[1]:>8} {vals[2]:>8}\n"

    ax4.text(
        0.5,
        0.5,
        summary_text,
        transform=ax4.transAxes,
        fontsize=11,
        fontfamily="monospace",
        verticalalignment="center",
        horizontalalignment="center",
        bbox=dict(boxstyle="round", facecolor="wheat", alpha=0.5),
    )

    output_path = output_dir / "dashboard.png"
    plt.savefig(output_path, dpi=150)
    plt.close()
    print(f"  ✅ Saved: {output_path}")


def main():
    if len(sys.argv) < 2:
        print("Usage: python visualize_benchmark.py <results_folder_or_file>")
        print("\nExamples:")
        print("  python visualize_benchmark.py results/")
        print(
            "  python visualize_benchmark.py results/benchmark_2024-12-02_15-30-00.json"
        )
        sys.exit(1)

    input_path = sys.argv[1]

    print("╔══════════════════════════════════════════════════════════════════╗")
    print("║          HTTP Benchmark Visualization Tool                       ║")
    print("╚══════════════════════════════════════════════════════════════════╝\n")

    # Load results
    print(f"📂 Loading results from: {input_path}")
    data = load_results(input_path)

    results = data.get("results", data)  # Support both new and old format
    metadata = data.get("metadata", {})

    # Create output directory
    input_path = Path(input_path)
    if input_path.is_file():
        output_dir = input_path.parent / "charts"
    else:
        output_dir = input_path / "charts"

    output_dir.mkdir(exist_ok=True)
    print(f"📊 Generating charts in: {output_dir}\n")

    # Generate all charts
    print("Creating visualizations...")
    create_timing_breakdown_chart(results, output_dir)
    create_latency_comparison_chart(results, output_dir)
    create_throughput_chart(results, output_dir)
    create_min_max_chart(results, output_dir)
    create_summary_dashboard(results, metadata, output_dir)

    print(f"\n✨ All charts saved to: {output_dir}")
    print("\nGenerated files:")
    for f in output_dir.glob("*.png"):
        print(f"  - {f.name}")


if __name__ == "__main__":
    main()
