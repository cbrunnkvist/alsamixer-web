#!/bin/bash

set -e

# Get version from git tag, or use "dev" if no tag
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Define platforms and architectures
PLATFORMS=("linux/amd64" "linux/arm64")

# Output directory
OUTPUT_DIR="dist"

# Create output directory if it doesn't exist
rm -rf $OUTPUT_DIR
mkdir -p $OUTPUT_DIR

# Build for each platform
for PLATFORM in "${PLATFORMS[@]}"; do
    echo "Building for $PLATFORM..."
    GOOS=$(echo $PLATFORM | cut -d/ -f1)
    GOARCH=$(echo $PLATFORM | cut -d/ -f2)

    BINARY_NAME="alsamixer-web-${VERSION}-linux-${GOARCH}"

    # Build the binary with stripped symbols and version injection
    GOOS=$GOOS GOARCH=$GOARCH go build \
        -ldflags="-s -w -X main.version=$VERSION" \
        -o "$OUTPUT_DIR/$BINARY_NAME" \
        ./cmd/alsamixer-web

    # Create checksums
    sha256sum "$OUTPUT_DIR/$BINARY_NAME" > "$OUTPUT_DIR/$BINARY_NAME.sha256"
    echo "Built $OUTPUT_DIR/$BINARY_NAME"
    echo "Checksum: $(cat "$OUTPUT_DIR/$BINARY_NAME.sha256")"
    echo ""
done

echo "Multi-platform build complete. Binaries are in $OUTPUT_DIR/"
