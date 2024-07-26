#!/bin/bash

INSTALL_DIR="/usr/local/simq/simd"
SIMD_BINARY="$INSTALL_DIR/simd/simd"
LIB_DIR="/var/lib/simd"
SERVICE_NAME="plato.simd.service"
SYSTEMD_SERVICE_FILE="/etc/systemd/system/$SERVICE_NAME"

#------------------------------------------------------------------------
# Step 1: Ensure the user has extracted the tar.gz file in /usr/local/
#------------------------------------------------------------------------
if [ ! -d "$INSTALL_DIR" ]; then
    echo "The directory $INSTALL_DIR does not exist. Please ensure you have extracted the distribution tar.gz file in /usr/local/."
    exit 1
fi

#------------------------------------------------------------------------
# Step 2: Check for the simq user and create if it doesn't exist
#------------------------------------------------------------------------
if id "simq" &>/dev/null; then
    echo "User 'simq' already exists."
else
    echo "Creating user 'simq'..."
    if ! sudo useradd -r -s /bin/false simq ; then
        echo "Failed to create user 'simq'. Exiting."
        exit 1
    fi
fi

#------------------------------------------------------------------------
# Step 3: Set up the necessary directory with correct permissions
#------------------------------------------------------------------------
echo "Setting up $LIB_DIR..."
mkdir -p "$LIB_DIR"
chown simq:simq "$LIB_DIR"
chmod 755 "$LIB_DIR"

#------------------------------------------------------------------------
# Step 4: Create the systemd service file
#------------------------------------------------------------------------
echo "Creating systemd service file..."
cat <<EOF | sudo tee "$SYSTEMD_SERVICE_FILE"
[Unit]
Description=PLATO SIMD Service
After=network.target

[Service]
Type=simple
User=simq
ExecStart=$SIMD_BINARY
WorkingDirectory=$LIB_DIR
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF

#------------------------------------------------------------------------
# Step 5: Enable and start the service
#------------------------------------------------------------------------
echo "Enabling and starting $SERVICE_NAME..."
systemctl daemon-reload
systemctl enable $SERVICE_NAME
systemctl start $SERVICE_NAME

echo "Installation complete. Check the status with 'systemctl status $SERVICE_NAME'."
