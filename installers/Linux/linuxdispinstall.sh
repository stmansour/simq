#!/bin/bash

LOG_FILE="/tmp/dispatcher_install.log"
DISPATCHER_DATA_DIR="/var/lib/dispatcher"
SIMQ_RELEASE_DIR="/usr/local/simq"
CONFIG_FILE="${SIMQ_RELEASE_DIR}/dispatcher/dispatcher.json5"
VERBOSE=0

# MySQL root credentials (adjust as necessary)
MYSQL_ROOT_USER="root"
MYSQL_ROOT_PASSWORD="root_password"

# Dispatcher MySQL credentials
DISPATCHER_MYSQL_USER="dispatcher"
DATABASE_NAME="simq"

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
    systemctl start dispatcher
    systemctl status dispatcher
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
# Make sure we are in the correct directory
#------------------------------------------------
if [ ! -d "./dispatcher" ]; then
    DIR=$(dirname "$0")
    vecho "Changing directory to $DIR..."
    cd "$DIR" || exit 1
    if [ ! -d "./dispatcher" ]; then
        echo "This script must be run from the directory containing linuxdispinstall.sh, dispatcher/, and bin/"
        exit 1
    fi
fi

#------------------------------------------------
# STOP DISPATCHER
#------------------------------------------------
if systemctl is-active --quiet dispatcher; then
    vecho "Attempting to stop dispatcher daemon..."
    #---------------------------------------
    # try to shutdown gracefully first
    #---------------------------------------
    systemctl stop dispatcher
    if systemctl is-active --quiet dispatcher; then
        #---------------------------------------
        # At this point, just kill it
        #---------------------------------------
        vecho "Failed to stop dispatcher service gracefully. Forcing shutdown..."
        sudo killall dispatcher
    else
        vecho "dispatcher service stopped successfully."
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
    sudo useradd -r -s /usr/sbin/nologin -c "DISPATCHER Service User" -m -d /var/empty dispatcher
    echo "dispatcher:Foolme123" | sudo chpasswd
fi

#------------------------------------------------
# Ensure the group 'dispatcher' exists
#------------------------------------------------
if getent group dispatcher &>/dev/null; then
    vecho "Group 'dispatcher' already exists."
else
    vecho "Creating group 'dispatcher'..."
    sudo groupadd dispatcher
fi

#------------------------------------------------
# Ensure user 'dispatcher' is in group 'dispatcher'
#------------------------------------------------
if id -nG dispatcher | grep -qw "dispatcher"; then
    vecho "User 'dispatcher' is already in group 'dispatcher'."
else
    vecho "Adding user 'dispatcher' to group 'dispatcher'..."
    sudo usermod -aG dispatcher dispatcher
fi

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
    "dispatcherSimulationsDir": "/var/lib/dispatcher",
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

#------------------------------------------------
# Setup MySQL user and database
#------------------------------------------------
vecho "Setting up MySQL user and database..."
mysql -u$MYSQL_ROOT_USER -p$MYSQL_ROOT_PASSWORD <<EOF
-- Create the database if it doesn't exist
CREATE DATABASE IF NOT EXISTS $DATABASE_NAME;

-- Create the dispatcher user if it doesn't exist and grant privileges
CREATE USER IF NOT EXISTS '$DISPATCHER_MYSQL_USER'@'localhost' IDENTIFIED BY '$DISPATCHER_MYSQL_PASSWORD';
GRANT ALL PRIVILEGES ON $DATABASE_NAME.* TO '$DISPATCHER_MYSQL_USER'@'localhost';

-- Apply the changes
FLUSH PRIVILEGES;
EOF

vecho "MySQL user and database setup completed."

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
