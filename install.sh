#!/bin/sh
set -e

# slack-export installer
# Usage: curl -fsSL https://raw.githubusercontent.com/ChrisEdwards/slack-export/main/install.sh | sh

REPO="ChrisEdwards/slack-export"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

# Detect OS
detect_os() {
    case "$(uname -s)" in
        Darwin*) echo "darwin" ;;
        Linux*)  echo "linux" ;;
        MINGW*|MSYS*|CYGWIN*) echo "windows" ;;
        *) echo "unsupported" ;;
    esac
}

# Detect architecture
detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64) echo "amd64" ;;
        arm64|aarch64) echo "arm64" ;;
        *) echo "unsupported" ;;
    esac
}

# Get latest release version
get_latest_version() {
    curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" |
        grep '"tag_name"' |
        sed -E 's/.*"([^"]+)".*/\1/'
}

main() {
    OS=$(detect_os)
    ARCH=$(detect_arch)

    if [ "$OS" = "unsupported" ] || [ "$ARCH" = "unsupported" ]; then
        echo "Error: Unsupported platform: $(uname -s) $(uname -m)"
        echo "Please download manually from https://github.com/${REPO}/releases"
        exit 1
    fi

    echo "Detecting platform... ${OS}-${ARCH}"

    VERSION=$(get_latest_version)
    if [ -z "$VERSION" ]; then
        echo "Error: Could not determine latest version"
        exit 1
    fi
    echo "Latest version: ${VERSION}"

    # Determine file extension
    if [ "$OS" = "windows" ]; then
        EXT="zip"
        ARCHIVE="slack-export-${VERSION}-${OS}-${ARCH}.${EXT}"
    else
        EXT="tar.gz"
        ARCHIVE="slack-export-${VERSION}-${OS}-${ARCH}.${EXT}"
    fi

    URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"

    echo "Downloading ${ARCHIVE}..."
    TMPDIR=$(mktemp -d)
    trap 'rm -rf "$TMPDIR"' EXIT

    curl -fsSL "$URL" -o "${TMPDIR}/${ARCHIVE}"

    echo "Extracting..."
    cd "$TMPDIR"
    if [ "$OS" = "windows" ]; then
        unzip -q "$ARCHIVE"
    else
        tar -xzf "$ARCHIVE"
    fi

    echo "Installing to ${INSTALL_DIR}..."
    if [ "$OS" = "windows" ]; then
        BINARY_EXT=".exe"
    else
        BINARY_EXT=""
    fi

    # Create install directory if needed
    mkdir -p "$INSTALL_DIR"

    mv "slack-export${BINARY_EXT}" "${INSTALL_DIR}/"
    mv "slackdump${BINARY_EXT}" "${INSTALL_DIR}/"

    echo ""
    echo "Installation complete!"
    echo ""

    # Check if INSTALL_DIR is in PATH
    case ":$PATH:" in
        *":$INSTALL_DIR:"*) ;;
        *)
            echo "Add ${INSTALL_DIR} to your PATH:"
            echo ""
            echo "  echo 'export PATH=\"${INSTALL_DIR}:\$PATH\"' >> ~/.zshrc"
            echo "  source ~/.zshrc"
            echo ""
            ;;
    esac

    echo "Next steps:"
    echo "  1. Authenticate with Slack:  slackdump auth"
    echo "  2. Run setup wizard:         slack-export init"
    echo "  3. Export your messages:     slack-export sync"
}

main
