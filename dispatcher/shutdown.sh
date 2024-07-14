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
    echo "Response: ${RESPONSE}" >> serverresponse
    if is_json "${RESPONSE}"; then
        echo "${RESPONSE}" | jq .
    else
        echo "${RESPONSE}"
    fi
}

#----------------------------------
# Shutdown the dispatcher
#----------------------------------
SHUTDOWN_CMD='{ "command": "Shutdown", "username": "test-user" }'
send_command "${SHUTDOWN_CMD}" "Shutdown"
sleep 2

#---------------------------------------
# Wait for the dispatcher to shutdown
#---------------------------------------
PID=$(pgrep dispatcher)
if [ "${PID}x" != "x" ]; then
    echo "Dispatcher still running after shutdown command" 
else
    echo "Dispatcher has shut down successfully"
fi

