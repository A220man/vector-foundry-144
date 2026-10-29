#!/bin/bash
set -e

TUIOS_BIN="/var/lib/alan-portfolio/workspaces/ryan/vector-foundry-144/tuios"

cleanup() {
    rm -rf /tmp/tuios
    pkill -f "tuios daemon" 2>/dev/null || true
}
trap cleanup EXIT

echo "=== Smoke Test for vector-foundry-144 ==="

# Test 1: version
echo "Test 1: version command"
$TUIOS_BIN version
echo "✓ version works"

# Test 2: new session
echo "Test 2: new session"
SESSION=$($TUIOS_BIN new smoke-test)
echo "Created session: $SESSION"
echo "✓ new session works"

# Test 3: list sessions
echo "Test 3: list sessions"
$TUIOS_BIN ls | grep -q "$SESSION"
echo "✓ list sessions works"

# Test 4: kill-server
echo "Test 4: kill-server"
$TUIOS_BIN kill-server
sleep 0.5
$TUIOS_BIN ls 2>&1 | grep -q "No daemon running"
echo "✓ kill-server works"

echo ""
echo "=== All smoke tests passed! ==="
