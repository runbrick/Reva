#!/bin/sh
set -eu

if [ "$(uname -s)" != "Darwin" ]; then
    echo "Run build.sh on macOS; use build.ps1 on Windows." >&2
    exit 1
fi
cd "$(dirname "$0")/.."
arch=${1:-$(go env GOHOSTARCH)}
case "$arch" in
    amd64|arm64) ;;
    *) echo "Architecture must be amd64 or arm64." >&2; exit 1 ;;
esac
output_dir="dist/darwin-$arch"
mkdir -p "$output_dir"

go tool viva build --target darwin --arch "$arch" -o "$output_dir/reqo"
go tool viva package --target darwin --arch "$arch" --format app,dmg --no-sign \
    --program "$output_dir/reqo" -o "$output_dir/Reqo.app"

print_size() {
    bytes=$1
    artifact=$2
    echo "$artifact: $bytes bytes"
}

print_size "$(stat -f %z "$output_dir/reqo")" "$output_dir/reqo"
# Sum file lengths, including the executable, Info.plist and generated .icns.
bundle_bytes=$(find "$output_dir/Reqo.app" -type f -exec stat -f %z {} + | awk '{sum += $1} END {printf "%.0f", sum}')
print_size "$bundle_bytes" "$output_dir/Reqo.app (all files)"
for image in "$output_dir"/reqo-*-darwin-"$arch".dmg; do
    print_size "$(stat -f %z "$image")" "$image"
done
