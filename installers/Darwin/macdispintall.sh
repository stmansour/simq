#!/bin/bash

LOG_FILE="/tmp/macinstall.log"
DISPATCHER_DATA_DIR="/var/lib/dispatcher"
SIMQ_RELEASE_DIR="/usr/local/simq"
CONFIG_FILE="${SIMQ_RELEASE_DIR}/dispatcher/dispatcherconf.json5"
VERBOSE=0

# Function to check if a string is valid JSON
#------------------------------------------------------------------------------
is_json() {
    echo "$1" | jq empty >/dev/null 2>&1
    return $?
}

# Function to send command and log response
#------------------------------------------------------------------------------
send_command() {
    local CMD=$1
    local RESPONSE
    curl -s -X POST http://localhost:8250/command -d "${CMD}" -H "Content-Type: application/json"
    echo "Response: ${RESPONSE}" >>serverresponse
    if is_json "${RESPONSE}"; then
        echo "${RESPONSE}" | jq .
    else
        echo "${RESPONSE}"
    fi
}

# Function to shutdown the dispatcher
#------------------------------------------------------------------------------
shutdown_dispatcher() {
    SHUTDOWN_CMD='{ "command": "Shutdown", "username": "installer" }'
    send_command "${SHUTDOWN_CMD}"
    sleep 1
}

#---------------------------------------------------------------
# Function to echo messages in verbose mode
#---------------------------------------------------------------
vecho() {
    if [ $VERBOSE -eq 1 ]; then
        echo "$@"
    fi
}

#------------------------------------------------
# Start dispatcher
#------------------------------------------------
start_dispatcher_service() {
    cd "${SIMQ_RELEASE_DIR}/dispatcher" || exit 1
    ./dispatcher &
    vecho "dispatcher started"
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
# Make sure we are in the correct directory.  The root directory of the distribution should
# have
#------------------------------------------------
if [ ! -d "./dispatcher" ]; then
    DIR=$(dirname "$0")
    vecho "Changing directory to $DIR..."
    cd "$DIR" || exit 1
    if [ ! -d "./dispatcher" ]; then
        echo "This script must be run from the directory containing macinstall.sh, dispatcher/, and bin/"
        exit 1
    fi
fi

#------------------------------------------------
# Stop any running instances of dispatcher
#------------------------------------------------
vecho "Checking for running instances of dispatcher..."
dispatchers=$(pgrep dispatcher)
if [ -n "$dispatchers" ]; then
    shutdown_dispatcher
    dispatchers=$(pgrep dispatcher)
    if [ -n "$dispatchers" ]; then
        vecho "Killing dispatcher instance..."
        killall dispatcher
        else
        vecho "Shutdown gracefully."
    fi
else
    vecho "No running instances of dispatcher found."
fi

#------------------------------------------------
# Now extract the updated files if needed...
#------------------------------------------------
if [ "$CWD" != "$SIMQ_RELEASE_DIR" ]; then
    vecho "Extracting files to $SIMQ_RELEASE_DIR..."
    mkdir -p $SIMQ_RELEASE_DIR
    cp -r ./dispatcher "$SIMQ_RELEASE_DIR"/
    cp -r ./bin "$SIMQ_RELEASE_DIR"/
fi

#------------------------------------------------
# Ensure that we have the dispatcher user and group...
#------------------------------------------------
if id "dispatcher" &>/dev/null; then
    vecho "User 'dispatcher' already exists."
else
    vecho "Creating user 'dispatcher'..."
    next_uid=$(dscl . -list /Users UniqueID | awk '{uid[$2]=1} END {for (i=501; i<600; i++) if (!uid[i]) {print i; exit}}')
    sudo dscl . -create /Users/dispatcher
    sudo dscl . -create /Users/dispatcher UserShell /usr/bin/false
    sudo dscl . -create /Users/dispatcher RealName "DISPATCHER Service User"
    sudo dscl . -create /Users/dispatcher UniqueID "$next_uid"
    sudo dscl . -create /Users/dispatcher PrimaryGroupID "$next_uid"
    sudo dscl . -create /Users/dispatcher NFSHomeDirectory /var/empty
    sudo dscl . -passwd /Users/dispatcher "Foolme123"
    sudo dscl . -append /Groups/wheel GroupMembership dispatcher
fi

#---------------------------------------------------------------
# Ensure the group 'dispatcher' exists and set the correct GID
#---------------------------------------------------------------
if ! dscl . -list /Groups PrimaryGroupID | grep -q "dispatcher"; then
    vecho "Creating group 'dispatcher' with GID $next_gid..."
    next_gid=$(dscl . -list /Groups PrimaryGroupID | awk '{gid[$2]=1} END {for (i=1000; i<60000; i++) if (!gid[i]) {print i; exit}}')
    sudo dscl . -create /Groups/dispatcher
    sudo dscl . -create /Groups/dispatcher PrimaryGroupID "$next_gid"
    sudo dscl . -append /Groups/dispatcher GroupMembership dispatcher
else
    vecho "Group 'dispatcher' already exists."
    next_gid=$(dscl . -read /Groups/dispatcher PrimaryGroupID | awk '{print $2}')
fi

#---------------------------------------------------------------
# Update PrimaryGroupID for user 'dispatcher' to match group 'dispatcher'
#---------------------------------------------------------------
vecho "Updating PrimaryGroupID for user 'dispatcher' to match group 'dispatcher'..."
sudo dscl . -create /Users/dispatcher PrimaryGroupID "$next_gid"

#------------------------------------------------
# Ensure /var/lib/dispatcher exists
#------------------------------------------------
if [ ! -d "$DISPATCHER_DATA_DIR" ]; then
    vecho "Creating $DISPATCHER_DATA_DIR..."
    sudo mkdir -p "$DISPATCHER_DATA_DIR"
    sudo chown -R dispatcher:dispatcher "$DISPATCHER_DATA_DIR"
fi

#---------------------------------------------------------------
# Set the owner of the simq released files to the dispatcher user
#---------------------------------------------------------------
vecho "Setting ownership of ${SIMQ_RELEASE_DIR} and ${DISPATCHER_DATA_DIR} to 'dispatcher'..."
sudo chown -R dispatcher:dispatcher "$SIMQ_RELEASE_DIR"
sudo chown -R dispatcher:dispatcher "$DISPATCHER_DATA_DIR"
sudo chmod -R 775 "$DISPATCHER_DATA_DIR"
sudo chmod -R 775 "$SIMQ_RELEASE_DIR"
sudo chmod u+s "${SIMQ_RELEASE_DIR}/dispatcher/dispatcher"

#------------------------------------------------
# Create the config file if needed
#------------------------------------------------
if [ ! -f "${DISPATCHER_DATA_DIR}/dispatcherconf.json5" ]; then
    CPUS=$(sysctl -n hw.ncpu)
    MEMORY=$(sysctl -n hw.memsize)
    MEMORY_GB=$(echo "scale=0; $MEMORY / (1024^3)" | bc)
    CPU_ARCH=$(uname -m)

    vecho "Creating config file $CONFIG_FILE..."
    cat <<EOF >"$CONFIG_FILE"
{
    "CPUs": $CPUS,
    "Memory": "${MEMORY_GB}GB",
    "CPUArchitecture": "$CPU_ARCH",
    "MaxSimulations": 1,
    "SimdSimulationsDir": "/var/lib/dispatcher",
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

# Prompt the user to start the dispatcher service
while true; do
    read -rp "Do you want to start the dispatcher service now? [Y/n] " response
    case $response in
    [Yy])
        start_dispatcher_service
        break
        ;;
    [Nn])
        echo "The dispatcher service will not be started."
        break
        ;;
    "")
        start_dispatcher_service
        break
        ;;
    *)
        echo "Please press y or n"
        ;;
    esac
done

echo "Installation complete!"
