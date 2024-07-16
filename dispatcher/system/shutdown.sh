#!/bin/bash

MYSQL=/usr/bin/mysql
if [ ! -f ${MYSQL} ]; then
    MYSQL=$(which mysql)
fi
echo "MySQL: ${MYSQL}"

#############################################################################
# pause()
#   Description:
#		Ask the user how to proceed.
#
#   Params:
#       none
#############################################################################
pause() {
	read -rp "Press [Enter] to continue, M to move ${2} to gold/${2}.gold, Q or X to quit..." x
	x=$(echo "${x}" | tr "[:upper:]" "[:lower:]")
	if [ "${x}" == "q" ] || [ "${x}" == "x" ]; then
		if [ "${MANAGESERVER}" -eq 1 ]; then
			echo "STOPPING DISPATCHER"
			pkill dispatcher
		fi
		exit 0
    fi
}

# Function to check if a string is valid JSON
#------------------------------------------------------------------------------
is_json() {
    echo "$1" | jq empty > /dev/null 2>&1
    return $?
}

# Function to send command and log response
#------------------------------------------------------------------------------
send_command() {
    local CMD=$1
    local DESCRIPTION=$2
    local RESPONSE
    echo "Sending ${DESCRIPTION} command: curl -s -X POST http://localhost:8250/command -d '${CMD}' -H 'Content-Type: application/json'"
    RESPONSE=$(curl -s -X POST http://localhost:8250/command -d "${CMD}" -H "Content-Type: application/json")
 #   echo "Response: ${RESPONSE}" >> serverresponse
    if is_json "${RESPONSE}"; then
        echo "${RESPONSE}" | jq .
    else
        echo "${RESPONSE}"
    fi
}

shutdownDispatcher() {
    #----------------------------------
    # Shutdown the dispatcher
    #----------------------------------
    SHUTDOWN_CMD='{ "command": "Shutdown", "username": "test-user" }'
    send_command "${SHUTDOWN_CMD}" "Shutdown"
    sleep 1

    #---------------------------------------
    # Wait for the dispatcher to shutdown
    #---------------------------------------
    PID=$(pgrep dispatcher)
    if [ "${PID}x" != "x" ]; then
        echo "Dispatcher still running after shutdown command" 
    else
        echo "Dispatcher has shut down successfully"
    fi
}


# Check the exit status of pgrep
pgrep dispatcher 2>&1 >/dev/null  # Redirect both standard output and error to /dev/null
if [[ $? -eq 0 ]]; then
    shutdownDispatcher
else
    # Check for empty standard error (might indicate process not found)
    if [[ -z "$(pgrep dispatcher 2>&1)" ]]; then
        echo "dispatcher was not running"
    else
        echo "Error: pgrep failed (check standard error for details)" >&2
        # Optional: Log captured standard error for further investigation
        # pgrep_error=$(pgrep dispatcher 2>&1)
        # echo "Standard Error: $pgrep_error" >&2
    fi
fi

#---------------------------------------
# KILL ALL SIMDs
#---------------------------------------
echo "Shutdown simd"
killall simd >/dev/null 2>&1

#---------------------------------------
# KILL ALL SIMULATORS
#---------------------------------------
echo "Killing all simulators"
killall simulator >/dev/null 2>&1

#---------------------------------------
# RESET SQL DB
#---------------------------------------
echo "resetting production simq.Queue table"
mysql simq <<EOF
DROP TABLE IF EXISTS Queue;
CREATE TABLE IF NOT EXISTS Queue (
     SID BIGINT AUTO_INCREMENT PRIMARY KEY,
     File VARCHAR(80) NOT NULL,
     Username VARCHAR(40) NOT NULL,
     Name VARCHAR(80) NOT NULL DEFAULT '',
     Priority INT NOT NULL DEFAULT 5,
     Description VARCHAR(256) NOT NULL DEFAULT '',
     MachineID VARCHAR(80) NOT NULL DEFAULT '',
     URL VARCHAR(80) NOT NULL DEFAULT '',
     State INT NOT NULL DEFAULT 0,
     DtEstimate DATETIME,
     DtCompleted DATETIME,
     Created TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
     Modified TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
EOF

#---------------------------------------
# RESET GENOME    (SIMULATION REPOSITORY)
#---------------------------------------
echo "resetting /genome/simres/2024/7/*"
rm -rf /genome/simres/2024/7/*

#---------------------------------------
# REMOVE DISPATCHER LOGS & TEMP STORAGE
#---------------------------------------
echo "emptying logs and temp storage for dispatcher"
rm -rf /var/lib/dispatcher/qdconfigs
rm -f /usr/local/simq/dispatcher/dispatcher.log

#---------------------------------------
# REMOVE SIMD LOGS & TEMP STORAGE
#---------------------------------------
echo "emptying logs and temp storage for simd"
rm -rf /var/lib/simd/simulations
rm -f /usr/local/simq/simd/simd.log
