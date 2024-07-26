#!/bin/bash
#--------------------------------------------------------------
# Set the base directory where your simq directory is located
#--------------------------------------------------------------
BASE_DIR="../../dist"

#--------------------------------------------------------------
# Extract the full version string from the psq binary
#--------------------------------------------------------------
FULL_VERSION=$("$BASE_DIR/simq/bin/psq" -v | awk '{print $3}')

#----------------------------------------------------------------------
# Extract only the major.minor version (removing everything after '-')
#----------------------------------------------------------------------
#VERSION=$(echo "$FULL_VERSION" | sed 's/-.*//')
VERSION="${FULL_VERSION%%-*}"

#--------------------------------------------------------------
# Check if version extraction was successful
#--------------------------------------------------------------
if [ -z "$VERSION" ]; then
    echo "Failed to extract major.minor version from psq binary."
    exit 1
fi

#--------------------------------------------------------------
# Run the pkgbuild command with the extracted version number
#--------------------------------------------------------------
pkgbuild --root "$BASE_DIR/simq" --identifier com.mpalfunds.simq --version "$VERSION" --install-location /usr/local/simq --scripts "$BASE_DIR/scripts" simq.pkg

echo "Package created with version $VERSION"
