#!/usr/bin/env python3
"""
plot.py — Generate breaking-point.png and latency.png from results.csv
Usage: python3 plot.py <results.csv> [output_dir]
"""

import csv
import sys
import os
import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import matplotlib.ticker as ticker

def load_csv(path):
    with open(path) as f:
        return list(csv.DictReader(f))

def main():
    if len(sys.argv) < 2:
        print("Usage: python3 plot.py results.csv [output_dir]")
        sys.exit(1)

    csv_path = sys.argv[1]
    out_dir  = sys.argv[2] if len(sys.argv) > 2 else os.path.dirname(os.path.abspath(csv_path))

    rows = load_csv(csv_path)
    rps        = [float(r["target_rps"])  for r in rows]
    error_pcts = [float(r["error_pct"])   for r in rows]
    p50        = [float(r["p50_ms"])      for r in rows]
    p95        = [float(r["p95_ms"])      for r in rows]
    p99        = [float(r["p99_ms"])      for r in rows]

    # Find breaking point: first step with any errors
    breaking_rps = None
    for r in rows:
        if float(r["error_pct"]) > 0:
            breaking_rps = float(r["target_rps"])
            break

    # ── Graph 1: Breaking Point ──────────────────────────────────────
    fig1, ax1 = plt.subplots(figsize=(10, 6))
    ax1.plot(rps, error_pcts, marker="o", linewidth=2.5,
             color="#E63946", label="Error rate (%)", zorder=3)
    ax1.fill_between(rps, error_pcts, alpha=0.15, color="#E63946")

    if breaking_rps is not None:
        ax1.axvline(x=breaking_rps, color="#F4A261", linewidth=2.0,
                    linestyle="--", label=f"Breaking point ({int(breaking_rps)} req/s)")
        ax1.annotate(
            f"Breaking point\n{int(breaking_rps)} req/s",
            xy=(breaking_rps, max(error_pcts) * 0.5),
            xytext=(breaking_rps + max(rps) * 0.04, max(error_pcts) * 0.6),
            fontsize=10, color="#F4A261",
            arrowprops=dict(arrowstyle="->", color="#F4A261", lw=1.5),
        )

    ax1.set_xlabel("Offered Request Rate (req/s)", fontsize=12)
    ax1.set_ylabel("Error Rate (%)", fontsize=12)
    ax1.set_title("goboxd Load Test — Breaking Point\n(MemoryHog.java, Java, 2 vCPU / 2 GB, 10 s timeout)",
                  fontsize=13, fontweight="bold")
    ax1.legend(fontsize=11)
    ax1.grid(True, alpha=0.3)
    ax1.set_xlim(left=0)
    ax1.set_ylim(bottom=0)
    ax1.yaxis.set_major_formatter(ticker.FormatStrFormatter("%.1f%%"))
    fig1.tight_layout()

    bp_path = os.path.join(out_dir, "breaking-point.png")
    fig1.savefig(bp_path, dpi=150, bbox_inches="tight")
    print(f"Saved: {bp_path}")

    # ── Graph 2: Latency Curve ───────────────────────────────────────
    fig2, ax2 = plt.subplots(figsize=(10, 6))
    ax2.plot(rps, p50, marker="o", linewidth=2.5, color="#2A9D8F", label="p50")
    ax2.plot(rps, p95, marker="s", linewidth=2.5, color="#E9C46A", label="p95")
    ax2.plot(rps, p99, marker="^", linewidth=2.5, color="#E63946", label="p99")

    if breaking_rps is not None:
        ax2.axvline(x=breaking_rps, color="#F4A261", linewidth=2.0,
                    linestyle="--", label=f"Breaking point ({int(breaking_rps)} req/s)")

    ax2.set_xlabel("Offered Request Rate (req/s)", fontsize=12)
    ax2.set_ylabel("Response Latency (ms)", fontsize=12)
    ax2.set_title("goboxd Load Test — Latency vs RPS\n(MemoryHog.java, Java, 2 vCPU / 2 GB, 10 s timeout)",
                  fontsize=13, fontweight="bold")
    ax2.legend(fontsize=11)
    ax2.grid(True, alpha=0.3)
    ax2.set_xlim(left=0)
    ax2.set_ylim(bottom=0)
    ax2.yaxis.set_major_formatter(ticker.FormatStrFormatter("%.0f ms"))
    fig2.tight_layout()

    lat_path = os.path.join(out_dir, "latency.png")
    fig2.savefig(lat_path, dpi=150, bbox_inches="tight")
    print(f"Saved: {lat_path}")

    # ── Summary ──────────────────────────────────────────────────────
    print()
    print("=== Summary ===")
    print(f"{'RPS':>6}  {'Throughput':>12}  {'Requests':>10}  {'Success':>8}  {'Failed':>8}  {'Error%':>8}  {'p50':>8}  {'p95':>8}  {'p99':>8}")
    for r in rows:
        print(f"{float(r['target_rps']):>6.0f}  {float(r['throughput_rps']):>12.2f}  {int(r['requests']):>10}  "
              f"{int(r['success']):>8}  {int(r['failed']):>8}  {float(r['error_pct']):>7.2f}%  "
              f"{float(r['p50_ms']):>7.0f}ms  {float(r['p95_ms']):>7.0f}ms  {float(r['p99_ms']):>7.0f}ms")
    if breaking_rps is not None:
        print(f"\nBreaking point: {int(breaking_rps)} req/s")
    else:
        print("\nBreaking point: NOT REACHED")

if __name__ == "__main__":
    main()
