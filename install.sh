#!/usr/bin/env sh
# Install the mrlib CLI from GitHub releases (preferred) or via `go install`.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/maedoc/mrlib/master/install.sh | sh
#   curl -fsSL https://raw.githubusercontent.com/maedoc/mrlib/master/install.sh | sh -s -- --version v1.0.0
#   curl -fsSL https://raw.githubusercontent.com/maedoc/mrlib/master/install.sh | sh -s -- --method go
#   curl -fsSL https://raw.githubusercontent.com/maedoc/mrlib/master/install.sh | sh -s -- --dest ~/.local/bin
set -eu

VERSION=""
METHOD="auto"
DEST=""
REPO="maedoc/mrlib"

while [ $# -gt 0 ]; do
    case "$1" in
        --version|-v) VERSION="${2:-}"; shift 2 ;;
        --method|-m) METHOD="${2:-}"; shift 2 ;;
        --dest|-d) DEST="${2:-}"; shift 2 ;;
        --repo) REPO="${2:-}"; shift 2 ;;
        *) echo "Unknown argument: $1" >&2; exit 1 ;;
    esac
done

if [ -z "$DEST" ]; then
    if [ -n "${GOBIN:-}" ]; then
        DEST="$GOBIN"
    elif [ -n "${GOPATH:-}" ] && [ -d "${GOPATH}/bin" ]; then
        DEST="${GOPATH}/bin"
    elif [ -w /usr/local/bin ]; then
        DEST=/usr/local/bin
    else
        DEST="${HOME}/.local/bin"
        mkdir -p "$DEST"
    fi
fi

detect_asset() {
    os="$(uname -s | tr '[:upper:]' '[:lower:]')"
    arch="$(uname -m)"
    case "$os" in
        linux) os=linux ;;
        darwin) os=darwin ;;
        mingw*|msys*|cygwin*) os=windows ;;
        *) echo "Unsupported OS: $os" >&2; return 1 ;;
    esac
    case "$arch" in
        x86_64|amd64) arch=amd64 ;;
        aarch64|arm64) arch=arm64 ;;
        *) echo "Unsupported arch: $arch" >&2; return 1 ;;
    esac
    ext=""
    [ "$os" = "windows" ] && ext=".exe"
    echo "mrlib-${os}-${arch}${ext}"
}

install_via_release() {
    asset="$(detect_asset)"
    if [ -z "$VERSION" ]; then
        VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1)"
    fi
    if [ -z "$VERSION" ]; then
        echo "Could not determine latest release version; pass --version vX.Y.Z" >&2
        return 1
    fi
    url="https://github.com/${REPO}/releases/download/${VERSION}/${asset}"
    echo "Downloading ${url}"
    tmp="$(mktemp)"
    curl -fsSL "$url" -o "$tmp"
    chmod +x "$tmp"
    mkdir -p "$DEST"
    mv "$tmp" "${DEST}/mrlib"
    echo "Installed mrlib ${VERSION} to ${DEST}/mrlib"
}

install_via_go() {
    if ! command -v go >/dev/null 2>&1; then
        echo "Go toolchain not found; install Go 1.23+ or use the release binary method" >&2
        return 1
    fi
    spec="github.com/${REPO}/go/cmd/mrlib@latest"
    if [ -n "$VERSION" ]; then
        spec="github.com/${REPO}/go/cmd/mrlib@${VERSION}"
    fi
    echo "Running: go install ${spec}"
    GOBIN="$DEST" go install "$spec"
    echo "Installed mrlib to ${DEST}/mrlib"
}

if [ "$METHOD" = "go" ]; then
    install_via_go
elif [ "$METHOD" = "release" ]; then
    install_via_release
else
    if command -v go >/dev/null 2>&1; then
        install_via_go || install_via_release
    else
        install_via_release
    fi
fi

case ":$PATH:" in
    *":$DEST:"*) ;;
    *) echo "Note: add $DEST to your PATH to use mrlib from anywhere" ;;
esac
