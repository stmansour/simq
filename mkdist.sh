#!/bin/bash

# Define the fixed part of the name
NAME="simq"

# Get the version from the simulator
VER=$(./dist/simq/bin/psq -v | awk '{print $3}')

# Determine the OS
UNAME_S=$(uname -s)
case "$UNAME_S" in
    Linux*)  OS="linux" ;;
    Darwin*) OS="macos" ;;
    *)       OS="unknown" ;;
esac

# Determine the CPU architecture
UNAME_M=$(uname -m)
case "$UNAME_M" in
    x86_64)  CPU="x86_64" ;;
    arm64)   CPU="arm64" ;;
    aarch64) CPU="arm64" ;;
    *)       CPU="unknown" ;;
esac

# Combine everything into the final tar file name
TARFILE="${NAME}.${VER}.${OS}.${CPU}.tar.gz"

# Create the tar file
echo "Creating tar file: $TARFILE"
cd ./dist || exit 1
tar -czvf "$TARFILE" simq

# Generate the SHA-256 checksum
if [ "$OS" == "macos" ]; then
    CHECKSUM=$(shasum -a 256 "$TARFILE" | awk '{print $1}')
else
    CHECKSUM=$(sha256sum "$TARFILE" | awk '{print $1}')
fi

# Print the result
if [ $? -eq 0 ]; then
    echo "Tar file created successfully: $TARFILE"
    echo "SHA-256 checksum: $CHECKSUM"
else
    echo "Failed to generate checksum for tar file: $TARFILE"
    exit 1
fi
