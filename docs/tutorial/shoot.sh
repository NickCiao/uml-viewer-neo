#!/bin/sh
# Re-shoot the tutorial's screenshots from examples/bakery. Run from the repo root.
set -e
CHROME="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
OUT=docs/tutorial
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

# A copy with no coverage report gives the first, unmeasured view.
rsync -a --exclude .umlv examples/bakery/ "$TMP/bakery/"
go run ./cmd/umlv --no-open "$TMP/bakery"
go run ./cmd/umlv --metrics --no-open examples/bakery >/dev/null

# shot NAME URL HEIGHT: a 1200px-wide window at 2x. The page centres the
# diagram at full size when it fits, so HEIGHT just trims the empty space.
shot() {
  "$CHROME" --headless=new --disable-gpu --hide-scrollbars --force-device-scale-factor=2 \
    --window-size=1200,"$3" --virtual-time-budget=10000 --screenshot="$PWD/$OUT/$1.png" "$2" 2>/dev/null
}
PAGE="file://$PWD/examples/bakery/.umlv/index.html"
shot unmeasured "file://$TMP/bakery/.umlv/index.html" 600
shot measured "$PAGE" 600
shot orders "$PAGE#f=orders" 340
shot card "$PAGE#s=report" 600
