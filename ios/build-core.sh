#!/bin/sh
# Build the shared Go core into the xcframework the iOS app links.
# The framework is a build product and is not tracked in git.

set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
out="$root/ios/Frameworks/MiddenCore.xcframework"

if ! command -v gomobile >/dev/null 2>&1; then
	if [ -x "$(go env GOPATH)/bin/gomobile" ]; then
		PATH="$(go env GOPATH)/bin:$PATH"
		export PATH
	else
		echo "gomobile not found: go install golang.org/x/mobile/cmd/gomobile@latest && gomobile init" >&2
		exit 1
	fi
fi

mkdir -p "$root/ios/Frameworks"
rm -rf "$out"
cd "$root"
gomobile bind -target ios -o "$out" ./mobile
test -d "$out" || { echo "gomobile produced no framework at $out" >&2; exit 1; }
echo "built $out"
