#!/bin/bash

LOG_FILE="/tmp/macinstall.log"
SIMD_DATA_DIR="/var/lib/simd"
SIMQ_RELEASE_DIR="/usr/local/simq"
CONFIG_FILE="${SIMQ_RELEASE_DIR}/simd/simdconf.json5"
VERBOSE=0

#---------------------------------------------------------------
# Function to echo messages in verbose mode
#---------------------------------------------------------------
vecho() {
    if [ $VERBOSE -eq 1 ]; then
        echo "$@"
    fi
}

#---------------------------------------------------------------
# Function to check if the user has write permissions
#
# INPUTS
# $1 - The path to check
#
# RETURNS - 0 if the user has write permissions, 1 otherwise
#---------------------------------------------------------------
canWrite() {
    local path=$1
    local X

    if [ -d "$path" ]; then
        # If it's a directory, check write permission
        if [ -w "$path" ]; then
            return 0
        else
            vecho "(p1) You do not have write permissions to $path"
            return 1
        fi
    elif [ -f "$path" ]; then
        # If it's a file, check if it exists and is writable
        if [ -w "$path" ]; then
            return 0
        else
            vecho "(p2) You do not have write permissions to $path"
            return 1
        fi
    else
        # If the path does not exist, check if the user can create a file in the directory
        local dir
        dir=$(dirname "$path")
        if [ -w "$dir" ]; then
            return 0
        else
            vecho "(p3) You do not have write permissions to $path"
            return 1
        fi
    fi
}

#------------------------------------------------
# Start simd
#------------------------------------------------
start_simd_service() {
    cd "${SIMQ_RELEASE_DIR}/simd" || exit 1
    ./simd &
    vecho "simd started"
}

#------------------------------------------------
# Process command-line options
#------------------------------------------------
while getopts "v" opt; do
    case $opt in
    v)
        VERBOSE=1
        ;;
    *)
        echo "Usage: $0 [-v]" >&2
        exit 1
        ;;
    esac
done

#--------------------------------------------------------------------------------------------
# Ensure the script is running as root. It needs to be root because there are commands that 
# create users and groups.
#--------------------------------------------------------------------------------------------
if [ "$EUID" -ne 0 ]; then
    echo "This script must be run as root. Please use sudo or run as root user."
    exit 1
fi

#------------------------------------------------
# Redirect all output to the log file
#------------------------------------------------
exec > >(tee -a "$LOG_FILE") 2>&1
vecho "-----------------------------------------------------------------------------"

#------------------------------------------------
# Make sure we are in the correct directory
#------------------------------------------------
if [ ! -d "./simd" ]; then
    DIR=$(dirname "$0")
    vecho "Changing directory to $DIR..."
    cd "$DIR" || exit 1
    if [ ! -d "./simd" ]; then
        echo "This script must be run from the directory containing macinstall.sh, simd/, and bin/"
        exit 1
    fi
fi

#------------------------------------------------
# Stop any running instances of psq and simd
#------------------------------------------------
vecho "Checking for running instances of psq..."
psqs=$(pgrep psq)
if [ -n "$psqs" ]; then
    COUNT=$(echo "$psqs" | wc -l)
    vecho "Found $COUNT instances of psq. Terminating..."
    killall psq
else
    vecho "No running instances of psq found."
fi

vecho "Checking for running instances of simd..."
simds=$(pgrep simd)
if [ -n "$simds" ]; then
    vecho "Found simd. Shutting down simd..."
    resp=$(curl -s http://localhost:8251/Shutdown)
    sleep 1
    simds=$(pgrep simd)
    if [ -n "$simds" ]; then
        vecho "Killing all remaining simd instances..."
        killall simd
    fi
else
    vecho "No running instances of simd found."
fi

#------------------------------------------------
# Now extract the updated files if needed...
#------------------------------------------------
if [ "$CWD" != "$SIMQ_RELEASE_DIR" ]; then
    vecho "Extracting files to $SIMQ_RELEASE_DIR..."
    mkdir -p $SIMQ_RELEASE_DIR
    cp -r ./simd "$SIMQ_RELEASE_DIR"/
    cp -r ./bin "$SIMQ_RELEASE_DIR"/
fi

#------------------------------------------------
# Ensure that we have the simd user and group...
#------------------------------------------------
if id "simd" &>/dev/null; then
    vecho "User 'simd' already exists."
else
    vecho "Creating user 'simd'..."
    sudo useradd -r -s /usr/sbin/nologin -c "SIMD Service User" -m -d /var/empty simd
    echo "simd:Foolme123" | sudo chpasswd
fi

#------------------------------------------------
# Ensure the group 'simd' exists
#------------------------------------------------
if getent group simd &>/dev/null; then
    vecho "Group 'simd' already exists."
else
    vecho "Creating group 'simd'..."
    sudo groupadd simd
fi

#------------------------------------------------
# Ensure user 'simd' is in group 'simd'
#------------------------------------------------
if id -nG simd | grep -qw "simd"; then
    vecho "User 'simd' is already in group 'simd'."
else
    vecho "Adding user 'simd' to group 'simd'..."
    sudo usermod -aG simd simd
fi

#------------------------------------------------
# Ensure /var/lib/simd exists
#------------------------------------------------
if [ ! -d "$SIMD_DATA_DIR" ]; then
    vecho "Creating $SIMD_DATA_DIR..."
    sudo mkdir -p "$SIMD_DATA_DIR"
    sudo chown -R simd:simd "$SIMD_DATA_DIR"
fi

#---------------------------------------------------------------
# Set the owner of the simq released files to the simd user
#---------------------------------------------------------------
vecho "Setting ownership of ${SIMQ_RELEASE_DIR} and ${SIMD_DATA_DIR} to 'simd'..."
sudo chown -R simd:simd "$SIMQ_RELEASE_DIR"
sudo chown -R simd:simd "$SIMD_DATA_DIR"
sudo chmod -R 775 "$SIMD_DATA_DIR"
sudo chmod -R 775 "$SIMQ_RELEASE_DIR"
sudo chmod u+s "${SIMQ_RELEASE_DIR}/simd/simd"

#------------------------------------------------
# Create the config file if needed
#------------------------------------------------
if [ ! -f "${CONFIG_FILE}" ]; then
    CPUS=$(nproc)
    MEMORY=$(grep MemTotal /proc/meminfo | awk '{print $2}')
    MEMORY_GB=$(echo "scale=0; $MEMORY / 1024 / 1024" | bc)
    CPU_ARCH=$(uname -m)

    vecho "Creating config file $CONFIG_FILE..."
    cat <<EOF >"$CONFIG_FILE"
{
    "CPUs": $CPUS,
    "Memory": "${MEMORY_GB}GB",
    "CPUArchitecture": "$CPU_ARCH",
    "MaxSimulations": 1,
    "SimdSimulationsDir": "/var/lib/simd",
    "DispatcherURL": "http://216.16.195.147:8250/"
}
EOF
    echo
    echo "**** PLEASE NOTE ****"
    echo "A default configuration file has been created at ${CONFIG_FILE}."
    echo "It is configured, by default, to allow the dispatcher to run 1 simulation on this machine."
    echo "Please edit this file to customize your configuration:   sudo vi ${CONFIG_FILE}"
    echo
fi

# Prompt the user to start the simd service
while true; do
    read -p "Do you want to start the simd service now? [Y/n] " response
    case $response in
    [Yy])
        start_simd_service
        break
        ;;
    [Nn])
        echo "The simd service will not be started."
        break
        ;;
    "")
        start_simd_service
        break
        ;;
    *)
        echo "Please press y or n"
        ;;
    esac
done

echo "Installation complete!"
