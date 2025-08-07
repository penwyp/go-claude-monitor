#!/bin/bash

# Test script for TUI fix
# This script runs the top command briefly and captures the initial output

echo "Testing TUI startup (will run for 2 seconds)..."
echo "Watch for any help page flash or display artifacts..."
echo ""
echo "Starting in 2 seconds..."
sleep 2

# Run the command with a TTY allocation
script -q /dev/null bash -c "./bin/go-claude-monitor top --plan max5 --timezone Asia/Shanghai --time-format 24h & sleep 2; kill %1" 2>/dev/null || true

echo ""
echo "Test completed. Did you see any help page flash or artifacts? (Check visually)"