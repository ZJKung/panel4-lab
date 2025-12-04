#!/bin/bash

# =============================================================================
# Network Traffic Control Script for macOS
# =============================================================================
# This script uses macOS's built-in dummynet (dnctl) and pf (pfctl) to simulate
# network conditions like packet loss and latency.
#
# HTTP/3 (QUIC) is designed to perform better under adverse network conditions:
# - Built-in loss recovery at the protocol level
# - No head-of-line blocking
# - Connection migration
#
# Usage:
#   sudo ./network_control.sh on [loss%] [delay_ms]  - Enable traffic shaping
#   sudo ./network_control.sh off                     - Disable traffic shaping
#   sudo ./network_control.sh status                  - Show current status
#
# Examples:
#   sudo ./network_control.sh on 5 50    # 5% packet loss, 50ms delay
#   sudo ./network_control.sh on 10      # 10% packet loss, no delay
#   sudo ./network_control.sh off        # Disable all rules
# =============================================================================

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
PF_CONF="/etc/pf.conf"
PF_ANCHOR="com.benchmark.dummynet"
BACKUP_CONF="/tmp/pf.conf.backup"

print_header() {
    echo ""
    echo -e "${BLUE}╔══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║        macOS Network Traffic Control for HTTP Benchmark      ║${NC}"
    echo -e "${BLUE}╚══════════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

print_status() {
    echo -e "${GREEN}[✓]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[!]${NC} $1"
}

print_error() {
    echo -e "${RED}[✗]${NC} $1"
}

check_root() {
    if [[ $EUID -ne 0 ]]; then
        print_error "This script must be run as root (use sudo)"
        exit 1
    fi
}

show_usage() {
    echo "Usage: sudo $0 <command> [options]"
    echo ""
    echo "Commands:"
    echo "  on [loss%] [delay_ms]  Enable traffic shaping"
    echo "                         loss%: Packet loss percentage (default: 5)"
    echo "                         delay_ms: Latency in milliseconds (default: 0)"
    echo "  off                    Disable traffic shaping"
    echo "  status                 Show current network control status"
    echo ""
    echo "Examples:"
    echo "  sudo $0 on 5 50        # 5% loss, 50ms delay"
    echo "  sudo $0 on 10          # 10% loss, no delay"
    echo "  sudo $0 on 2 100       # 2% loss, 100ms delay"
    echo "  sudo $0 off            # Disable all rules"
    echo "  sudo $0 status         # Check current status"
    echo ""
    echo "Recommended settings to demonstrate HTTP/3 advantages:"
    echo "  - Light:  sudo $0 on 2 50    (2% loss, 50ms delay)"
    echo "  - Medium: sudo $0 on 5 100   (5% loss, 100ms delay)"
    echo "  - Heavy:  sudo $0 on 10 150  (10% loss, 150ms delay)"
}

enable_traffic_control() {
    local LOSS_PERCENT=${1:-5}
    local DELAY_MS=${2:-0}

    print_header
    echo -e "Enabling traffic control with:"
    echo -e "  ${YELLOW}Packet Loss:${NC} ${LOSS_PERCENT}%"
    echo -e "  ${YELLOW}Delay:${NC} ${DELAY_MS}ms"
    echo ""

    # Convert percentage to probability (0.0 - 1.0)
    LOSS_PROB=$(echo "scale=4; $LOSS_PERCENT / 100" | bc)

    # Step 1: Flush existing dummynet pipes
    print_status "Flushing existing dummynet rules..."
    dnctl -q flush 2>/dev/null || true

    # Step 2: Create dummynet pipes
    # Pipe 1: For outgoing traffic
    # Pipe 2: For incoming traffic
    print_status "Creating dummynet pipes..."

    if [[ $DELAY_MS -gt 0 ]]; then
        dnctl pipe 1 config delay ${DELAY_MS}ms plr $LOSS_PROB
        dnctl pipe 2 config delay ${DELAY_MS}ms plr $LOSS_PROB
    else
        dnctl pipe 1 config plr $LOSS_PROB
        dnctl pipe 2 config plr $LOSS_PROB
    fi

    # Step 3: Create pf anchor rules
    print_status "Creating pf anchor rules..."

    # Create anchor file
    cat > /tmp/dummynet_anchor.conf << EOF
# Dummynet rules for HTTP benchmark testing
# Apply to all outgoing TCP/UDP traffic (except localhost)
dummynet out proto tcp from any to ! 127.0.0.1 pipe 1
dummynet out proto udp from any to ! 127.0.0.1 pipe 1
dummynet in proto tcp from ! 127.0.0.1 to any pipe 2
dummynet in proto udp from ! 127.0.0.1 to any pipe 2
EOF

    # Step 4: Backup current pf.conf
    if [[ -f "$PF_CONF" ]]; then
        cp "$PF_CONF" "$BACKUP_CONF"
        print_status "Backed up pf.conf to $BACKUP_CONF"
    fi

    # Step 5: Check if anchor already exists in pf.conf
    if ! grep -q "dummynet-anchor.*$PF_ANCHOR" "$PF_CONF" 2>/dev/null; then
        print_status "Adding dummynet anchor to pf.conf..."

        # Create a temporary pf.conf with our anchor
        cat > /tmp/pf_modified.conf << EOF
# Added by network_control.sh for HTTP benchmark
dummynet-anchor "$PF_ANCHOR"
anchor "$PF_ANCHOR"

EOF

        # Append original pf.conf content
        if [[ -f "$PF_CONF" ]]; then
            cat "$PF_CONF" >> /tmp/pf_modified.conf
        fi

        cp /tmp/pf_modified.conf "$PF_CONF"
    fi

    # Step 6: Load the anchor rules
    print_status "Loading dummynet anchor rules..."
    pfctl -a "$PF_ANCHOR" -f /tmp/dummynet_anchor.conf 2>/dev/null

    # Step 7: Enable pf
    print_status "Enabling pf..."
    pfctl -e 2>/dev/null || true

    echo ""
    echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${GREEN}Traffic control ENABLED${NC}"
    echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"
    echo ""
    echo "Current dummynet pipes:"
    dnctl list
    echo ""
    print_warning "Remember to run 'sudo $0 off' when done testing!"
    echo ""
    echo "Why HTTP/3 performs better with packet loss:"
    echo "  1. No TCP head-of-line blocking - lost packets don't block other streams"
    echo "  2. QUIC's built-in loss recovery is more efficient than TCP"
    echo "  3. 0-RTT connection resumption reduces latency impact"
    echo ""
}

disable_traffic_control() {
    print_header
    print_status "Disabling traffic control..."

    # Step 1: Flush dummynet pipes
    print_status "Flushing dummynet pipes..."
    dnctl -q flush 2>/dev/null || true

    # Step 2: Flush pf anchor
    print_status "Flushing pf anchor..."
    pfctl -a "$PF_ANCHOR" -F all 2>/dev/null || true

    # Step 3: Optionally restore original pf.conf
    if [[ -f "$BACKUP_CONF" ]]; then
        print_status "Restoring original pf.conf..."
        cp "$BACKUP_CONF" "$PF_CONF"
    fi

    # Step 4: Reload pf with default rules
    print_status "Reloading pf..."
    pfctl -f "$PF_CONF" 2>/dev/null || true

    echo ""
    echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${GREEN}Traffic control DISABLED${NC}"
    echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"
    echo ""
}

show_status() {
    print_header

    echo -e "${YELLOW}Dummynet Pipes:${NC}"
    dnctl list 2>/dev/null || echo "  No pipes configured"
    echo ""

    echo -e "${YELLOW}PF Status:${NC}"
    pfctl -s info 2>/dev/null | head -5 || echo "  PF not available"
    echo ""

    echo -e "${YELLOW}PF Anchor Rules ($PF_ANCHOR):${NC}"
    pfctl -a "$PF_ANCHOR" -s rules 2>/dev/null || echo "  No anchor rules"
    echo ""
}

# Main script
case "${1:-}" in
    on|enable)
        check_root
        enable_traffic_control "${2:-5}" "${3:-0}"
        ;;
    off|disable)
        check_root
        disable_traffic_control
        ;;
    status)
        show_status
        ;;
    *)
        show_usage
        exit 1
        ;;
esac
