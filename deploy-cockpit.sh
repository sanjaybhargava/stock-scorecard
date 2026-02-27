#!/bin/bash
set -euo pipefail

# Usage:
#   ./deploy-cockpit.sh ZY7393 BT2632     # specific clients
#   ./deploy-cockpit.sh --all              # scan data/ for cockpit configs

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
COCKPIT_REPO="$HOME/stock-cockpit"
BINARY="$SCRIPT_DIR/stock-scorecard-arm64"

# Also deploy to vimal-stock-scorecard for backward compat
VIMAL_REPO="$HOME/vimal-stock-scorecard"

# ── Parse client list ─────────────────────────────────────
CLIENTS=()
if [ "${1:-}" = "--all" ]; then
    for cfg in "$SCRIPT_DIR"/data/*/cockpit_*.json; do
        # Extract client ID from filename: cockpit_ZY7393.json → ZY7393
        basename=$(basename "$cfg")
        id="${basename#cockpit_}"
        id="${id%.json}"
        CLIENTS+=("$id")
    done
    if [ ${#CLIENTS[@]} -eq 0 ]; then
        echo "ERROR: No cockpit configs found in data/"
        exit 1
    fi
elif [ $# -gt 0 ]; then
    CLIENTS=("$@")
else
    echo "Usage: ./deploy-cockpit.sh ZY7393 BT2632"
    echo "       ./deploy-cockpit.sh --all"
    exit 1
fi

echo "Clients: ${CLIENTS[*]}"
echo ""

# ── Step 0: Fetch stock TRI ──────────────────────────────
echo "=== Step 0: Fetch stock TRI ==="
python3 "$SCRIPT_DIR/scripts/fetch_stock_tri.py"

# ── Step 1: Generate cockpit data for each client ────────
echo ""
echo "=== Step 1: Generate cockpit data ==="

if [ ! -f "$BINARY" ]; then
    echo "Building binary..."
    cd "$SCRIPT_DIR"
    GOARCH=arm64 go build -o "$BINARY" ./cmd/scorecard
fi

JSON_FILES=()
for CLIENT in "${CLIENTS[@]}"; do
    echo ""
    echo "--- Generating cockpit for $CLIENT ---"
    python3 "$SCRIPT_DIR/scripts/build_cockpit_data.py" --client "$CLIENT" 2>/dev/null || true
    "$BINARY" cockpit --client "$CLIENT" --data "$SCRIPT_DIR/data"
    JSON_FILES+=("cockpit_${CLIENT}.json")
done

# ── Step 2: Build UI ─────────────────────────────────────
echo ""
echo "=== Step 2: Build UI ==="
cd "$SCRIPT_DIR/ui-cockpit"
npm install --silent
npm run build

# ── Step 3: Deploy to stock-cockpit ──────────────────────
echo ""
echo "=== Step 3: Deploy to stock-cockpit ==="

if [ ! -d "$COCKPIT_REPO" ]; then
    echo "Creating $COCKPIT_REPO..."
    mkdir -p "$COCKPIT_REPO"
    cd "$COCKPIT_REPO"
    git init
    git checkout -b gh-pages
    git remote add origin "git@github.com:sanjaybhargava/stock-cockpit.git" 2>/dev/null || true
fi

# Copy UI build
rm -rf "$COCKPIT_REPO"/assets "$COCKPIT_REPO"/index.html
cp -r "$SCRIPT_DIR/ui-cockpit/dist/"* "$COCKPIT_REPO/"

# Copy all cockpit JSON files
for JSON in "${JSON_FILES[@]}"; do
    cp "$SCRIPT_DIR/$JSON" "$COCKPIT_REPO/"
done

cd "$COCKPIT_REPO"
git add -A
git commit -m "Deploy cockpit: ${CLIENTS[*]}" || echo "No changes to commit"
git push -u origin gh-pages || echo "Push failed — check remote setup"

# ── Step 4: Backward compat — update vimal-stock-scorecard ──
if [ -d "$VIMAL_REPO" ] && [ -f "$SCRIPT_DIR/cockpit_ZY7393.json" ]; then
    echo ""
    echo "=== Step 4: Update vimal-stock-scorecard (backward compat) ==="

    # Build UI with vimal base path
    cd "$SCRIPT_DIR/ui-cockpit"
    VITE_BASE="/vimal-stock-scorecard/cockpit/" npm run build

    rm -rf "$VIMAL_REPO/cockpit"
    cp -r "$SCRIPT_DIR/ui-cockpit/dist" "$VIMAL_REPO/cockpit"
    # Copy with client-specific name (UI fetches cockpit_{clientId}.json)
    cp "$SCRIPT_DIR/cockpit_ZY7393.json" "$VIMAL_REPO/cockpit/cockpit_ZY7393.json"
    # Also keep cockpit.json for backward compat
    cp "$SCRIPT_DIR/cockpit_ZY7393.json" "$VIMAL_REPO/cockpit/cockpit.json"

    cd "$VIMAL_REPO"
    git add cockpit/
    git commit -m "Update cockpit" || echo "No changes to commit"
    git push || echo "Push failed"
fi

echo ""
echo "=== Done! ==="
echo "Live at: https://sanjaybhargava.github.io/stock-cockpit/?client=${CLIENTS[0]}"
