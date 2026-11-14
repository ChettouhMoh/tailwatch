#!/bin/sh
# The installed VPS needs neither of the build tools (Go and dpkg-deb).
set -eu
if [ "$#" -ne 3 ]; then
    echo "Usage: sh deploy/build-deb.sh VERSION amd64|arm64 OUTPUT.deb" >&2
    exit 2
fi
version=$1
arch=$2
output=$3
case "$arch" in amd64|arm64) ;; *) echo "Unsupported architecture: $arch" >&2; exit 2 ;; esac
dpkg --validate-version "$version"
repo=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
tree=$(mktemp -d)
trap 'rm -rf "$tree"' EXIT
chmod 0755 "$tree"
install -d "$tree/DEBIAN" "$tree/usr/bin" "$tree/etc/tailwatch" "$tree/usr/lib/systemd/system"
if [ -n "${TAILWATCH_BINARY:-}" ]; then
    # Accept a Go binary from a separate build host after checking its target.
    metadata=$(go version -m "$TAILWATCH_BINARY")
    for value in "path tailwatch/cmd/tailwatch" "build GOOS=linux" "build GOARCH=$arch"; do
        printf '%s\n' "$metadata" | awk '{print $1, $2}' | grep -Fxq "$value" || {
            echo "Expected a Tailwatch binary for linux/$arch" >&2
            exit 1
        }
    done
    install -m 0755 "$TAILWATCH_BINARY" "$tree/usr/bin/tailwatch"
else
    (
        cd "$repo"
        CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath -o "$tree/usr/bin/tailwatch" ./cmd/tailwatch
    )
fi
install -m 0755 "$repo/deploy/tailwatch-setup" "$tree/usr/bin/tailwatch-setup"
install -m 0644 "$repo/config.example.json" "$tree/etc/tailwatch/config.json"
install -m 0644 "$repo/deploy/tailwatch.service" "$tree/usr/lib/systemd/system/"
install -D -m 0644 "$repo/README.md" "$tree/usr/share/doc/tailwatch/README.md"
for script in postinst prerm postrm; do
    install -m 0755 "$repo/deploy/debian/$script" "$tree/DEBIAN/"
done
printf '%s\n' /etc/tailwatch/config.json > "$tree/DEBIAN/conffiles"
cat > "$tree/DEBIAN/control" <<EOF
Package: tailwatch
Version: $version
Architecture: $arch
Section: admin
Priority: optional
Maintainer: Tailwatch maintainers <noreply@github.com>
Depends: systemd (>= 249), init-system-helpers, python3 (>= 3.8), ca-certificates
Description: SSH login notifications for Linux servers
 Monitors successful OpenSSH and Tailscale SSH logins in the system journal.
EOF
dpkg-deb --root-owner-group --build "$tree" "$output"
