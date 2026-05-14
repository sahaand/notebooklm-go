#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHROME="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
PROFILE_DIR="$HOME/.chrome-notebooklm"

if [ ! -f "$CHROME" ]; then
  echo "Chrome not found at: $CHROME"
  exit 1
fi

echo "Launching Chrome with remote debugging..."
"$CHROME" \
  --remote-debugging-port=9222 \
  --user-data-dir="$PROFILE_DIR" \
  https://notebooklm.google.com > /dev/null 2>&1 &

CHROME_PID=$!

sleep 2
echo ""
echo ">>> Chrome is open. Once you are logged in to NotebookLM, press Enter to export cookies..."
read -r

echo "Exporting cookies..."
cd "$SCRIPT_DIR"

if [ ! -d "node_modules/ws" ]; then
  echo "Installing ws dependency..."
  npm install ws --save 2>&1 | tail -3
fi

node export_cookies.js 2>&1

echo "Done. You can close Chrome now."
