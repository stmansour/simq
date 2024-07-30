#!/bin/bash

LOG_FILE="/tmp/macinstall.log"
SIMD_DATA_DIR="/var/lib/simd"
SIMQ_RELEASE_DIR="/usr/local/simq"
CONFIG_FILE="${SIMQ_RELEASE_DIR}/simd/simdconf.json5"

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

    if [ -d "$path" ]; then
        # If it's a directory, check write permission
        if [ -w "$path" ]; then
            return 0
        else
            return 1
        fi
    elif [ -f "$path" ]; then
        # If it's a file, check if it exists and is writable
        if [ -w "$path" ]; then
            return 0
        else
            return 1
        fi
    else
        # If the path does not exist, check if the user can create a file in the directory
        local dir
        dir=$(dirname "$path")
        if [ -w "$dir" ]; then
            return 0
        else
            return 1
        fi
    fi
}

#------------------------------------------------
# Start simd
#------------------------------------------------
start_simd_service() {
    cd ${SIMQ_RELEASE_DIR}/simd || exit 1
    ./simd &
    echo "simd started"
}

#------------------------------------------------
# first check if the user has write permissions
#------------------------------------------------
if ! canWrite "/usr/local/simq" || ! canWrite "/usr/local" || ! canWrite "/var/lib/" || ! canWrite "/usr/local/simq/simd"; then
    echo "You do not have the necessary write permissions to install SimQ. Please re-run the installer with sudo."
    exit 1
fi

#------------------------------------------------
# Redirect all output to the log file
#------------------------------------------------
exec > >(tee -a "$LOG_FILE") 2>&1
echo "-----------------------------------------------------------------------------"

#------------------------------------------------
# Stop any running instances of psq and simd
#------------------------------------------------
echo -n "Checking for running instances of psq...  "
psqs=$(pgrep psq)
if [ "X${psqs}" != "X" ]; then
    COUNT=$(echo $psqs | wc -l)
    echo "found $COUNT instances.  Terminating..."
    killall psq
else
    echo "none found"
fi

echo -n "Checking for running instances of simd...  "
simds=$(pgrep simd)
if [ "X${simds}" != "X" ]; then
    echo "found. Shutting down simd..."
    resp=$(curl http://localhost:8251/Shutdown)
    sleep 1
    simds=$(pgrep simd)
    if [ "X${simds}" != "X" ]; then
        echo "killing all simd instances"
        killall simd
    fi
else
    echo "none found"
fi

#------------------------------------------------
# Now extract the updated files...
#------------------------------------------------
tar xvf simq.*.tar.gz -C /usr/local

#------------------------------------------------
# Ensure that we have the simd user and group...
#------------------------------------------------
if id "simd" &>/dev/null; then
    echo "User 'simd' already exists."
else
    echo "Creating user 'simd'..."
    next_uid=$(dscl . -list /Users UniqueID | awk '{uid[$2]=1} END {for (i=501; i<600; i++) if (!uid[i]) {print i; exit}}')
    sudo dscl . -create /Users/simd
    sudo dscl . -create /Users/simd UserShell /usr/bin/false
    sudo dscl . -create /Users/simd RealName "SIMD Service User"
    sudo dscl . -create /Users/simd UniqueID "$next_uid"
    sudo dscl . -create /Users/simd PrimaryGroupID "$next_uid"
    sudo dscl . -create /Users/simd NFSHomeDirectory /var/empty
    sudo dscl . -passwd /Users/simd "Foolme123"
    sudo dscl . -append /Groups/wheel GroupMembership simd
fi

#---------------------------------------------------------------
# Ensure the group 'simd' exists and set the correct GID
#---------------------------------------------------------------
if ! dscl . -list /Groups PrimaryGroupID | grep -q "simd"; then
    echo "Creating group 'simd' with GID $next_gid..."
    next_gid=$(dscl . -list /Groups PrimaryGroupID | awk '{gid[$2]=1} END {for (i=1000; i<60000; i++) if (!gid[i]) {print i; exit}}')
    sudo dscl . -create /Groups/simd
    sudo dscl . -create /Groups/simd PrimaryGroupID "$next_gid"
    sudo dscl . -append /Groups/simd GroupMembership simd
else
    echo "Group 'simd' already exists."
    next_gid=$(dscl . -read /Groups/simd PrimaryGroupID | awk '{print $2}')
fi

#---------------------------------------------------------------
# Update PrimaryGroupID for user 'simd' to match group 'simd'
#---------------------------------------------------------------
echo "Updating PrimaryGroupID for user 'simd' to match group 'simd'..."
sudo dscl . -create /Users/simd PrimaryGroupID "$next_gid"

#------------------------------------------------
# Ensure /var/lib/simd exists
#------------------------------------------------
if [ ! -d "$SIMD_DATA_DIR" ]; then
    echo "Creating $SIMD_DATA_DIR..."
    sudo mkdir -p "$SIMD_DATA_DIR"
    sudo chown -R simd:simd "$SIMD_DATA_DIR"
fi

#---------------------------------------------------------------
# Set the owner of the simq released files to the simd user
# Allow for group write access to make life easier for the
# user who needs to look after these files. They should add
# themselves to group 'simd'.
#---------------------------------------------------------------
echo "Setting ownership of ${SIMQ_RELEASE_DIR} and ${SIMD_DATA_DIR} to 'simd'..."
sudo chown -R simd:simd "$SIMQ_RELEASE_DIR"
sudo chown -R simd:simd "$SIMD_DATA_DIR"
sudo chmod -R 775 "$SIMD_DATA_DIR"
sudo chmod -R 775 "$SIMQ_RELEASE_DIR"
sudo chmod u+s "${SIMQ_RELEASE_DIR}/simd/simd"

#------------------------------------------------
# Create the config file if needed
#------------------------------------------------
if [ ! -f "${SIMD_DATA_DIR}/simdconf.json5" ]; then
    CPUS=$(sysctl -n hw.ncpu)
    MEMORY=$(sysctl -n hw.memsize)
    MEMORY_GB=$(echo "scale=0; $MEMORY / (1024^3)" | bc)
    CPU_ARCH=$(uname -m)

    echo "Creating config file $CONFIG_FILE..."
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
