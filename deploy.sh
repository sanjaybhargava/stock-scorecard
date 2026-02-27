#!/usr/bin/env bash
set -euo pipefail

# deploy.sh — Build and deploy stock-scorecard to GitHub Pages
#
# Prerequisites:
#   - Go 1.21+, Node.js 18+, git, gh CLI
#   - Tradebook CSVs in ~/Downloads/
#   - NIFTY 500 TRI CSV in ~/Downloads/
#
# Usage:
#   ./deploy.sh
#   ./deploy.sh --skip-scorecard   # Skip Go build, reuse existing scorecard.json
#
# Note: This script uses legacy mode (all-in-one flags).
# The new two-command workflow is:
#   stock-scorecard import --source ~/Downloads --tri ~/Downloads/NIFTY500_TRI_Indexed.csv --output ./data/
#   stock-scorecard score --data ./data/ --client BT2632 --output ./ui/public/scorecard.json

REPO_ROOT="$(cd "$(dirname "$0")" && pwd)"
UI_DIR="$REPO_ROOT/ui"
DIST_DIR="$UI_DIR/dist"
SCORECARD_JSON="$UI_DIR/public/scorecard.json"

SKIP_SCORECARD=false
if [[ "${1:-}" == "--skip-scorecard" ]]; then
    SKIP_SCORECARD=true
fi

echo "=== stock-scorecard deploy ==="

# Step 1: Build Go CLI and generate scorecard JSON
if [[ "$SKIP_SCORECARD" == true ]]; then
    echo "[1/4] Skipping scorecard generation (--skip-scorecard)"
    if [[ ! -f "$SCORECARD_JSON" ]]; then
        echo "ERROR: $SCORECARD_JSON not found. Run without --skip-scorecard first."
        exit 1
    fi
else
    echo "[1/4] Building Go CLI and generating scorecard..."
    cd "$REPO_ROOT"
    go build -o stock-scorecard ./cmd/scorecard
    # Generate dividends CSV if not already present
    DIVIDENDS_CSV="$REPO_ROOT/dividends.csv"
    if [[ ! -f "$DIVIDENDS_CSV" ]]; then
        echo "       Generating preliminary scorecard for dividend lookup..."
        ./stock-scorecard \
            --tradebooks ~/Downloads \
            --tri ~/Downloads/NIFTY500_TRI_Indexed.csv \
            --output "$SCORECARD_JSON"
        echo "       Pulling dividend data..."
        python3 "$REPO_ROOT/scripts/pull_dividends.py" \
            --scorecard "$SCORECARD_JSON" \
            --output "$DIVIDENDS_CSV"
    fi
    ./stock-scorecard \
        --tradebooks ~/Downloads \
        --tri ~/Downloads/NIFTY500_TRI_Indexed.csv \
        --dividends "$DIVIDENDS_CSV" \
        --fno ~/Downloads \
        --output "$SCORECARD_JSON"
    echo "       Scorecard written to $SCORECARD_JSON"
fi

# Step 2: Build React UI
echo "[2/4] Building React UI..."
cd "$UI_DIR"
npm install --silent
npm run build
echo "       UI built to $DIST_DIR"

# Step 3: Verify build output
echo "[3/4] Verifying build..."
if [[ ! -f "$DIST_DIR/index.html" ]]; then
    echo "ERROR: index.html not found in $DIST_DIR"
    exit 1
fi
if [[ ! -f "$DIST_DIR/scorecard.json" ]]; then
    echo "ERROR: scorecard.json not found in $DIST_DIR (should be copied from public/)"
    exit 1
fi
echo "       Build verified: index.html + scorecard.json present"

# Step 4: Deploy to gh-pages
echo "[4/4] Deploying to gh-pages..."
cd "$REPO_ROOT"

# Save current branch
CURRENT_BRANCH=$(git branch --show-current)

# Create a temporary directory for the deploy
DEPLOY_TMP=$(mktemp -d)
cp -r "$DIST_DIR"/* "$DEPLOY_TMP/"

# Switch to gh-pages, wipe, copy fresh build
if git show-ref --verify --quiet refs/heads/gh-pages; then
    git checkout gh-pages
else
    git checkout --orphan gh-pages
fi

# Remove everything except .git
git rm -rf . 2>/dev/null || true
find . -maxdepth 1 ! -name '.git' ! -name '.' -exec rm -rf {} + 2>/dev/null || true

# Copy build output
cp -r "$DEPLOY_TMP"/* .

# Commit and push
git add -A
git commit -m "Deploy to GitHub Pages" --allow-empty
git push origin gh-pages --force

# Return to original branch
git checkout "$CURRENT_BRANCH"

# Cleanup
rm -rf "$DEPLOY_TMP"

echo ""
echo "=== Deployed! ==="
echo "    https://sanjaybhargava.github.io/stock-scorecard/"
