#!/usr/bin/env bash
set -euo pipefail

# deploy-friend.sh — Deploy a friend's scorecard + cockpit to GitHub Pages
#
# Usage:
#   ./deploy-friend.sh DUA527
#   ./deploy-friend.sh DUA527 --scorecard-only
#
# Prerequisites:
#   - Friend's scorecard_<ID>.json and cockpit_<ID>.json in ~/Downloads
#   - GitHub repos: stock-scorecard (gh-pages), stock-cockpit (gh-pages)

if [ $# -lt 1 ]; then
    echo "Usage: ./deploy-friend.sh <CLIENT_ID> [--scorecard-only]"
    echo ""
    echo "Expects in ~/Downloads:"
    echo "  scorecard_<ID>.json   (required)"
    echo "  cockpit_<ID>.json     (optional)"
    exit 1
fi

CLIENT_ID="$1"
SCORECARD_ONLY=false
if [ "${2:-}" = "--scorecard-only" ]; then
    SCORECARD_ONLY=true
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DOWNLOADS="$HOME/Downloads"
SCORECARD_JSON="$DOWNLOADS/scorecard_${CLIENT_ID}.json"
COCKPIT_JSON="$DOWNLOADS/cockpit_${CLIENT_ID}.json"

# ── Validate inputs ──────────────────────────────────────
if [ ! -f "$SCORECARD_JSON" ]; then
    echo "ERROR: $SCORECARD_JSON not found"
    echo "Ask your friend to send their scorecard_${CLIENT_ID}.json"
    exit 1
fi
echo "Found: $SCORECARD_JSON"

HAS_COCKPIT=false
if [ -f "$COCKPIT_JSON" ]; then
    HAS_COCKPIT=true
    echo "Found: $COCKPIT_JSON"
elif [ "$SCORECARD_ONLY" = false ]; then
    echo "Note: No cockpit_${CLIENT_ID}.json found (will deploy scorecard only)"
fi

# ── Step 1: Deploy scorecard to stock-scorecard gh-pages ──
echo ""
echo "=== Deploying scorecard for $CLIENT_ID ==="

cd "$SCRIPT_DIR"

# Build UI (with ?client= support)
echo "[1/3] Building scorecard UI..."
cd "$SCRIPT_DIR/ui"
npm install --silent
npm run build

# Prepare deploy
DEPLOY_TMP=$(mktemp -d)
cp -r "$SCRIPT_DIR/ui/dist/"* "$DEPLOY_TMP/"

# Copy this client's scorecard JSON
cp "$SCORECARD_JSON" "$DEPLOY_TMP/scorecard_${CLIENT_ID}.json"

# Also keep the default scorecard.json if it exists in public/
if [ -f "$SCRIPT_DIR/ui/public/scorecard.json" ]; then
    cp "$SCRIPT_DIR/ui/public/scorecard.json" "$DEPLOY_TMP/scorecard.json"
fi

# Copy any other client scorecards already deployed (preserve existing)
cd "$SCRIPT_DIR"
CURRENT_BRANCH=$(git branch --show-current)

if git show-ref --verify --quiet refs/heads/gh-pages; then
    # Extract existing client JSONs from gh-pages
    for f in $(git ls-tree --name-only gh-pages | grep '^scorecard_.*\.json$' || true); do
        # Don't overwrite the one we're deploying
        if [ "$f" != "scorecard_${CLIENT_ID}.json" ]; then
            git show "gh-pages:$f" > "$DEPLOY_TMP/$f" 2>/dev/null || true
        fi
    done
fi

# Switch to gh-pages and deploy
if git show-ref --verify --quiet refs/heads/gh-pages; then
    git checkout gh-pages
else
    git checkout --orphan gh-pages
fi

git rm -rf . 2>/dev/null || true
find . -maxdepth 1 ! -name '.git' ! -name '.' -exec rm -rf {} + 2>/dev/null || true

cp -r "$DEPLOY_TMP"/* .
git add -A
git commit -m "Deploy scorecard: $CLIENT_ID" --allow-empty
git push origin gh-pages --force

git checkout "$CURRENT_BRANCH"
rm -rf "$DEPLOY_TMP"

SCORECARD_URL="https://sanjaybhargava.github.io/stock-scorecard/?client=${CLIENT_ID}"
echo "[2/3] Scorecard deployed!"
echo "  → $SCORECARD_URL"

# ── Step 2: Deploy cockpit to stock-cockpit gh-pages ──────
COCKPIT_URL=""
if [ "$HAS_COCKPIT" = true ] && [ "$SCORECARD_ONLY" = false ]; then
    echo ""
    echo "=== Deploying cockpit for $CLIENT_ID ==="

    COCKPIT_REPO="$HOME/stock-cockpit"

    # Build cockpit UI
    echo "[2/3] Building cockpit UI..."
    cd "$SCRIPT_DIR/ui-cockpit"
    npm install --silent
    npm run build

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

    # Copy this client's cockpit JSON
    cp "$COCKPIT_JSON" "$COCKPIT_REPO/cockpit_${CLIENT_ID}.json"

    cd "$COCKPIT_REPO"
    git add -A
    git commit -m "Deploy cockpit: $CLIENT_ID" || echo "No changes to commit"
    git push -u origin gh-pages || echo "Push failed — check remote setup"

    COCKPIT_URL="https://sanjaybhargava.github.io/stock-cockpit/?client=${CLIENT_ID}"
    echo "[3/3] Cockpit deployed!"
    echo "  → $COCKPIT_URL"
fi

# ── Summary ──────────────────────────────────────────────
echo ""
echo "=== Done! Send these URLs to your friend ==="
echo ""
echo "  Realised:    $SCORECARD_URL"
if [ -n "$COCKPIT_URL" ]; then
    echo "  Unrealised:  $COCKPIT_URL"
fi
echo ""
