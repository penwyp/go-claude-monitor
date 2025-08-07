#!/bin/bash

# Visual test for TUI fix
# This test runs the TUI briefly and lets you observe if the help page flashes

echo "================================"
echo "Visual TUI Test"
echo "================================"
echo ""
echo "This test will run the TUI for 3 seconds."
echo "Watch carefully for:"
echo "  1. Help page briefly appearing at startup"
echo "  2. Display artifacts or residual text"
echo "  3. Clean transition to loading screen"
echo ""
echo "Press Enter to start the test..."
read

# Run the command
echo "Starting TUI..."
./bin/go-claude-monitor top --plan max5 --timezone Asia/Shanghai --time-format 24h &
PID=$!

# Let it run for 3 seconds
sleep 3

# Kill the process
kill $PID 2>/dev/null

echo ""
echo "Test completed."
echo ""
echo "Did you observe any issues? (y/n)"
read response

if [ "$response" = "y" ]; then
    echo "Please describe what you saw:"
    read description
    echo "Issue reported: $description"
else
    echo "Great! The TUI appears to be working correctly."
fi