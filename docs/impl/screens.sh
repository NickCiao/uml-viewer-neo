#!/bin/sh
# Re-shoot the screenshots the visual review looks at. Run from the repo root
# after `go run ./cmd/umlv --metrics --no-open .` and
# `go run ./cmd/umlv --no-open ../spaced-repetition`.
set -e
CHROME="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
OUT=docs/impl/screens
mkdir -p "$OUT"
shot() {
  "$CHROME" --headless=new --disable-gpu --hide-scrollbars --window-size=1600,1000 \
    --virtual-time-budget=10000 --screenshot="$PWD/$OUT/$1.png" "$2" 2>/dev/null
}
NEO="file://$PWD/.umlv/index.html"
SR="file://$(cd ../spaced-repetition && pwd)/.umlv/index.html"
shot sr-top "$SR"
shot sr-routes "$SR#f=routes"
shot sr-selected "$SR#s=db"
shot sr-focus "$SR#s=db&a=off"
shot neo-top "$NEO"
shot neo-module-card "$NEO#f=internal&s=internal/metrics"
shot neo-folder-card "$NEO#s=internal"
sips -m "/System/Library/ColorSync/Profiles/Generic Gray Profile.icc" \
  "$OUT/neo-top.png" --out "$OUT/neo-top-grey.png" >/dev/null
sips -m "/System/Library/ColorSync/Profiles/Generic Gray Profile.icc" \
  "$OUT/neo-module-card.png" --out "$OUT/neo-module-card-grey.png" >/dev/null
