#!/bin/bash
RUNSINGLETEST=0
TESTCOUNT=0
ERRORCOUNT=0

MYSQL=/usr/bin/mysql
if [ ! -f ${MYSQL} ]; then
    MYSQL=/usr/local/bin/mysql
fi

usage() {
    cat <<EOF

SYNOPSIS
	$0 [-a -t]

	Run the tests and compare the output of each test step to its associated
    known-good output. If they miscompare, fail and stop the script. If they
    match, keep going until all tasks are completed.

OPTIONS
	-a  If a test fails, pause after showing diffs from gold files, prompt
	    for what to do next:  [Enter] to continue, m to move the output file
	    into gold/ , or Q / X to exit.

    -h  display this help information.

	-t  Sets the environment variable RUNSINGLETEST to the supplied value. By
	    default, "${RUNSINGLETEST}x" == "x" and this should cause all of the
	    tests in the script to run. But if you would like to be able to run
	    an individual test by name, you can use ${RUNSINGLETEST} to check and
	    see if the user has requested a specific test.
EOF
}

# Function to send command and log response
#------------------------------------------------------------------------------
send_command() {
    local CMD=$1
    local DESCRIPTION=$2
    echo "Sending ${DESCRIPTION} command: ${CMD}" >> ${RESFILE}
    local RESPONSE=$(curl -s -X POST http://localhost:8250/command -d "${CMD}" -H "Content-Type: application/json")
    echo "Response: ${RESPONSE}" >> serverresponse
    echo "${RESPONSE}" | jq . >> ${RESFILE}
}

#------------------------------------------------------------------------------
# Function to compare a report file to its gold standard
# INPUTS
#    $1 = name of un-normalized output file
#    $2 = if supplied, it means that there will be more tests where
#         the output needs to be listed. So use "echo -n" on the output
#------------------------------------------------------------------------------
compareToGold() {
    local reportFile=$1
    local goldFile="gold/${reportFile}.gold"
    local normalizedFile="${reportFile}.normalized"

    # if it's a csv file, delete to the first blank line...
    if [[ ${reportFile} =~ \.csv$ ]]; then
        awk 'flag; /^$/ {flag=1}' "${reportFile}" >"${reportFile}.tmp" && mv "${reportFile}.tmp" "${reportFile}"
    fi

    # Normalize the report file
    sed -E \
        -e 's/Created":.*/CREATED_PLACEHOLDER/' \
        -e 's/Modified":.*/MODIFIED_PLACEHOLDER/' \
        -e 's/dispatcher with PID:.*/dispatcher with PROCESSID_PLACEHOLDER/' \
        -e 's/Current Time:.*/Current Time: TIME_PLACEHOLDER/' \
        -e 's/Random number seed:[[:space:]]+[0-9]+/Random number seed: SEED_PLACEHOLDER/' \
        -e 's/Archive directory:.*/Archive directory: PLACEHOLDER/' \
        -e 's/Elapsed time:.*/Archive directory: PLACEHOLDER/' \
        -e 's/ - [0-9a-zA-Z-]{64}/ - GUID/' \
        "$reportFile" >"$normalizedFile"

    # Check if running on Windows
    if [[ "$(uname -s)" =~ MINGW|CYGWIN|MSYS ]]; then
        echo "Detected Windows OS. Normalizing line endings for ${normalizedFile}."

        # Use sed to replace CRLF with LF, output to temp file
        sed 's/\r$//' "${normalizedFile}" >"${goldFile}.tmp"
        goldFile="${goldFile}.tmp"
    fi

    # Compare the normalized report to the gold standard
    if diff "${normalizedFile}" "${goldFile}"; then
        echo  "PASSED"
        rm "${normalizedFile}"
    else
        echo "Differences detected.  meld ${normalizedFile} ${goldFile}"
        ((ERRORCOUNT++))
        # Prompt the user for action
        if [[ "${ASKBEFOREEXIT}" == 1 ]]; then
            while true; do
                read -rp "Choose action - Continue (C), Move (M), or eXit (X) [C]: " choice
                choice=${choice:-C} # Default to 'C' if no input
                case "$choice" in
                C | "")
                    echo "Continuing..."
                    return 0
                    ;;
                M | m)
                    echo "Moving normalized file to gold standard..."
                    mv "$normalizedFile" "$goldFile"
                    return 0
                    ;;
                X | x)
                    echo "Exiting..."
                    exit 1
                    ;;
                *) echo "Invalid choice. Please choose C, M, or X." ;;
                esac
            done
        fi
    fi
}

###############################################################################
#    INPUT
###############################################################################
while getopts "at:" o; do
    echo "o = ${o}"
    case "${o}" in
    a)
        ASKBEFOREEXIT=1
        echo "WILL ASK BEFORE EXITING ON ERROR"
        ;;
    t)
        SINGLETEST="${OPTARG}"
        echo "SINGLETEST set to ${SINGLETEST}"
        ;;
    *)
        usage
        exit 1
        ;;
    esac
done
shift $((OPTIND - 1))
############################################################################################

killall dispatcher      # precaution

#------------------------------------------------------------------------------
#  TEST a
#  initial dispatcher test
#------------------------------------------------------------------------------
TFILES="a"
STEP=0
if [[ "${SINGLETEST}${TFILES}" = "${TFILES}" || "${SINGLETEST}${TFILES}" = "${TFILES}${TFILES}" ]]; then
    echo -n "Test ${TFILES} - Basic dispatcher test... "

    RESFILE="${TFILES}${STEP}"
    echo "Result File: ${RESFILE}" > ${RESFILE}

    #-------------------------------------------------------
    # start a new dispatcher with a clean database table
    #-------------------------------------------------------
    echo "DROP TABLE IF EXISTS Queue;" | ${MYSQL} simq
    ./dispatcher >DISPATCHER.log 2>&1 &
    DISPATCHER_PID=$!
    echo "Started dispatcher with PID: ${DISPATCHER_PID}" >> ${RESFILE}
    sleep 2

    #----------------------------------
    # Check if dispatcher is running
    #----------------------------------
    if ! ps -p ${DISPATCHER_PID} > /dev/null; then
        echo "Dispatcher failed to start"
        exit 1
    fi

    #----------------------------------
    # Send commands to the dispatcher
    #----------------------------------
    # Add a new simulation
    ADD_CMD='{ "command": "NewSimulation", "username": "test-user", "data": { "file": "path/to/simulation.tar.gz", "name": "Test Simulation", "priority": 5, "description": "A test simulation", "url": "http://localhost:8080" } }'
    send_command "${ADD_CMD}" "NewSimulation"
    sleep 1

    #----------------------------------
    # Get the active queue
    #----------------------------------
    GET_QUEUE_CMD='{ "command": "GetActiveQueue", "username": "test-user" }'
    send_command "${GET_QUEUE_CMD}" "GetActiveQueue"
    sleep 1
    #----------------------------------
    # Delete the simulation
    #---------------------------------- 
    DELETE_CMD='{ "command": "DeleteItem", "username": "test-user", "data": { "sid": 1 } }'
    send_command "${DELETE_CMD}" "DeleteItem"
    sleep 1

    #----------------------------------
    # Shutdown the server
    #----------------------------------
    SHUTDOWN_CMD='{ "command": "Shutdown", "username": "test-user" }'
    echo "Shutting down the server with command: ${SHUTDOWN_CMD}"  >> ${RESFILE}
    SHUTDOWN_RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" -d "${SHUTDOWN_CMD}" http://localhost:8250/command)
    echo "Shutdown Response: ${SHUTDOWN_RESPONSE}" >> ${RESFILE}

    #---------------------------------------
    # Wait for the dispatcher to shutdown
    #---------------------------------------
    sleep 2
    if ps -p ${DISPATCHER_PID} > /dev/null; then
        echo "Dispatcher still running after shutdown command" >> ${RESFILE}
    else
        echo "Dispatcher has shut down successfully" >> ${RESFILE}
    fi
    compareToGold ${RESFILE}
    ((TESTCOUNT++))
fi

echo "Total tests: ${TESTCOUNT}"
echo "Total errors: ${ERRORCOUNT}"
if [ "${ERRORCOUNT}" -gt 0 ]; then
    exit 2
fi

