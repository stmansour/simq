#!/bin/bash

# Define the fixed part of the name
NAME="simq"
# Get the version from psq
VER=$(./dist/simq/bin/psq -v | awk '{print $3}')
# Extract Major.Minor version
MAJOR_MINOR=$(echo "$VER" | cut -d'-' -f1)

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

# Function to create the archive
create_archive() {
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
}

# Function to post the archive
post_archive() {
    # Define the file pattern to remove (using wildcard for timestamp)
    FILE_PATTERN="${NAME}.${MAJOR_MINOR}-*.$OS.$CPU.tar.gz"
    POSTNAME="${NAME}.${VER}.${OS}.${CPU}"
    echo "Removing old files matching: $FILE_PATTERN"
    echo "POSTNAME: $POSTNAME"
    
    # Check if running on the local machine 'plato'
    if [ "$(hostname)" = "plato" ]; then
        # Remove old files and copy new ones locally
        rm /var/www/html/downloads/$FILE_PATTERN
        cp "./dist/${POSTNAME}.tar.gz" /var/www/html/downloads/
    else
        # Remove old files and copy new ones remotely
        ssh steve@plato "rm /var/www/html/downloads/$FILE_PATTERN"
        scp -i ~/.ssh/id_platosrv "./dist/${POSTNAME}.tar.gz" steve@plato:/var/www/html/downloads/
    fi
    
    echo "Copied new files to /var/www/html/downloads/"
}

# Parse command-line options
while getopts ":cp" opt; do
    case ${opt} in
        c )
            create_archive
            ;;
        p )
            post_archive
            ;;
        \? )
            echo "Usage: $0 [-c] [-p]"
            echo "  -c    Create the archive"
            echo "  -p    Post the archive"
            exit 1
            ;;
    esac
done

# If no options were passed, display usage
if [ $OPTIND -eq 1 ]; then
    echo "Usage: $0 [-c] [-p]"
    echo "  -c    Create the archive"
    echo "  -p    Post the archive"
    exit 1
fi
