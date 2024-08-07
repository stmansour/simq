#!/bin/bash
RUNSINGLETEST=0
TESTCOUNT=0
ERRORCOUNT=0
DISPATCHEROFF=0
DISPATCHER_RUNNING=0
LOADTESTDATAONLY=0
RESDIR=$(grep "SimResultsDir" dispatcher.json5 | sed 's/".*"://' | sed 's/[", ]//g')
QDIR=$(grep "DispatcherQueueDir" dispatcher.json5 | sed 's/".*"://' | sed 's/[", ]//g')
MYSQL=$(which mysql)

echo "MySQL: ${MYSQL}"

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

    -d  Do not run the dispatcher.  This is useful if you just want to run the
        dispatcher in the debugger and use this program's infrastructure to
        setup the data based on some of the test cases.

    -k  Just load the data for the specified test case (use with -t).  This is
        useful if you want to test dispatcher with a different program, like
        psq.

    -h  display this help information.

    -t  Sets the environment variable RUNSINGLETEST to the supplied value. By
        default, "${RUNSINGLETEST}x" == "x" and this should cause all of the
        tests in the script to run. But if you would like to be able to run
        an individual test by name, you can use ${RUNSINGLETEST} to check and
        see if the user has requested a specific test.

EXAMPLES
    $0
    Run all the tests

    $0 -a
    Run all the tests and pause after any failures to allow you to exit or
    even change the gold files to match your output.

    $0 -t b
    Run only test b
    
    $0 -k c
    Just set up the data and directories with the data for test c.
EOF
}

#############################################################################
# pause()
#   Description:
#		Ask the user how to proceed.
#
#   Params:
#       none
#############################################################################
pause() {
    read -rp "Press [Enter] to continue, M to move gold, Q or X to quit..." x
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
    echo "$1" | jq empty >/dev/null 2>&1
    return $?
}

# Function to send command and log response
#------------------------------------------------------------------------------
send_command() {
    local CMD=$1
    local DESCRIPTION=$2
    local RESPONSE
    echo "Sending ${DESCRIPTION} command: curl -s -X POST http://localhost:8250/command -d '${CMD}' -H 'Content-Type: application/json'" >>${RESFILE}
    RESPONSE=$(curl -s -X POST http://localhost:8250/command -d "${CMD}" -H "Content-Type: application/json")
    echo "Response: ${RESPONSE}" >>serverresponse
    if is_json "${RESPONSE}"; then
        echo "${RESPONSE}" | jq . >>${RESFILE}
    else
        echo "${RESPONSE}" >>${RESFILE}
    fi
}

# Function to send command with file and log response
#------------------------------------------------------------------------------
send_command_with_file() {
    local DESCRIPTION=$1
    local CMD=$2
    local FILE=$3
    local RESPONSE

    # Compose the full command
    CMD_FULL=$(jq -n --argjson data "${CMD}" \
        '{command: "NewSimulation", username: "test-user", data: $data}')

    # Encode CMD_FULL as a string to include in the curl command
    CMD_FULL_STR=$(echo "${CMD_FULL}" | jq -c .)
    echo "Sending ${DESCRIPTION} command with file: curl -s -X POST http://localhost:8250/command -F 'data=${CMD_FULL_STR}' -F 'file=@${FILE}'" >>${RESFILE}
    RESPONSE=$(curl -s -X POST http://localhost:8250/command -F "data=${CMD_FULL_STR}" -F "file=@${FILE}")
    echo "Response: ${RESPONSE}" >>serverresponse
    if is_json "${RESPONSE}"; then
        echo "${RESPONSE}" | jq . >>${RESFILE}
    else
        echo "${RESPONSE}" >>${RESFILE}
    fi
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
        -e 's/qdconfigs.*json5/TMPFILE_PLACEHOLDER/' \
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
        echo "PASSED"
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
#------------------------------------------------------------------------------
# setupData - sets up all directories and database for test $TFILES
# INPUTS
#    none yet
#------------------------------------------------------------------------------
setupData() {
    # echo "Setting up data for ${TFILES}"
    #---------------------------
    # first clean directories
    #---------------------------
    rm -rf "${RESDIR:?}/"*
    rm -rf "${QDIR:?}/"*
    #---------------------------
    # clean database
    #---------------------------
    if [ -f testdata/"${TFILES}"/simq.sql ]; then
        ${MYSQL} simqtest <testdata/"${TFILES}"/simq.sql
    else
        echo "DROP TABLE IF EXISTS Queue;" | ${MYSQL} simqtest
        rm -rf qdconfigs
    fi
    #---------------------------
    # Initialize datafiles
    #---------------------------
    if [ -f testdata/"${TFILES}"/res.tar ]; then
        tar xf "testdata/${TFILES}/res.tar" -C "${RESDIR}"
    fi
    if [ -f testdata/"${TFILES}"/qdata.tar ]; then
        tar xf "testdata/${TFILES}/qdata.tar" -C "${QDIR}"
    fi
}

#------------------------------------------------------------------------------
# startDispatcher - kill any existing dispatcher, then start the dispatcher.
# INPUTS
#    none yet
#------------------------------------------------------------------------------
startDispatcher() {
    if ((DISPATCHER_RUNNING == 0 && DISPATCHEROFF == 0)); then
        killall -9 dispatcher >/dev/null 2>&1
        ./dispatcher >DISPATCHER.log 2>&1 &
        DISPATCHER_PID=$!
        sleep 2
        DISPATCHER_RUNNING=1
    fi
}

###############################################################################
#    INPUT
###############################################################################
while getopts "adkt:" o; do
    echo "o = ${o}"
    case "${o}" in
    a)
        ASKBEFOREEXIT=1
        echo "WILL ASK BEFORE EXITING ON ERROR"
        ;;
    d)
        DISPATCHEROFF=1
        echo "WILL NOT RUN DISPATCHER IN THIS TEST. YOU MUST RUN SEPARATELY"
        ;;

    k)  LOADTESTDATAONLY=1
        echo "LOADING TEST DATA ONLY"
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
if (( LOADTESTDATAONLY == 1 )); then
    TFILES="${SINGLETEST}"
    setupData
    exit 0
fi
############################################################################################

#------------------------------------------------------------------------------
#  TEST a
#  initial dispatcher test
#------------------------------------------------------------------------------
TFILES="a"
STEP=0
if [[ "${SINGLETEST}${TFILES}" = "${TFILES}" || "${SINGLETEST}${TFILES}" = "${TFILES}${TFILES}" ]]; then
    setupData
    startDispatcher
    echo -n "Test ${TFILES} - Basic dispatcher test... "

    RESFILE="${TFILES}${STEP}"
    echo "Result File: ${RESFILE}" >${RESFILE}

    #----------------------------------
    # Create a new simulation
    #----------------------------------
    ADD_CMD='{"name":"Test Simulation","priority":5,"description":"A test simulation","url":"http://localhost:8080","OriginalFilename":"config.json5"}'
    send_command_with_file "NewSimulation" "${ADD_CMD}" "config.json5"
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
    echo "Shutting down the server with command: ${SHUTDOWN_CMD}" >>${RESFILE}
    SHUTDOWN_RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" -d "${SHUTDOWN_CMD}" http://localhost:8250/command)
    echo "Shutdown Response: ${SHUTDOWN_RESPONSE}" >>${RESFILE}

    #---------------------------------------
    # Wait for the dispatcher to shutdown
    #---------------------------------------
    sleep 2
    if ps -p "${DISPATCHER_PID}" >/dev/null; then
        echo "Dispatcher still running after shutdown command" >>${RESFILE}
    else
        echo "Dispatcher has shut down successfully" >>${RESFILE}
        DISPATCHER_RUNNING=0
    fi
    compareToGold ${RESFILE}
    ((TESTCOUNT++))
fi

#------------------------------------------------------------------------------
#  TEST b
#  book a simulation
#------------------------------------------------------------------------------
TFILES="b"
STEP=0
if [[ "${SINGLETEST}${TFILES}" = "${TFILES}" || "${SINGLETEST}${TFILES}" = "${TFILES}${TFILES}" ]]; then
    if [ ! -f config.json5 ]; then
        echo "Missing config.json5. Please copy config.json5 to this directory and try again."
        exit 1
    fi
    setupData
    startDispatcher
    echo -n "Test ${TFILES} - Book a simulation... "

    RESFILE="${TFILES}${STEP}"
    echo "Result File: ${RESFILE}" >${RESFILE}

    #----------------------------------
    # Create a new simulation
    #----------------------------------
    ADD_CMD='{"name":"Test Simulation","priority":5,"description":"A test simulation","url":"http://localhost:8080","OriginalFilename":"config.json5"}'
    send_command_with_file "NewSimulation" "${ADD_CMD}" "config.json5"
    sleep 2 # Small delay to ensure the command is processed

    #----------------------------------
    # Book the simulation
    #----------------------------------
    BOOK_CMD='{"command":"Book","username":"test-user","data":{"MachineID":"test-machine","CPUs":10,"Memory":"64GB","CPUArchitecture":"ARM64","Availability":"always"}}'
    echo "Sending Book command: ${BOOK_CMD}" >>${RESFILE}
    RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" -d "${BOOK_CMD}" http://localhost:8250/command)
    sleep 1 # Ensure the response is fully captured

    # Handle multipart response
    BOUNDARY=$(echo "${RESPONSE}" | grep -o 'boundary=[^;]*' | cut -d '=' -f 2)
    echo "${RESPONSE}" | awk -v boundary="${BOUNDARY}" -v resfile="${RESFILE}" -v tfiles="${TFILES}" '
    BEGIN { RS="--"boundary; FS="\r\n\r\n" }
    /Content-Disposition: form-data; name="json"/ {
        print "JSON Response: "$2 >> resfile
    }
    /Content-Disposition: form-data; name="file"/ {
        getline; getline;
        filename = tfiles ".rcvd.config.json5"
        print $0 > filename
        print "Received config file part, saved as " filename >> resfile
    }'
    sleep 1

    #----------------------------------
    # Get the active queue and verify state
    #----------------------------------
    GET_QUEUE_CMD='{ "command": "GetActiveQueue", "username": "test-user" }'
    send_command "${GET_QUEUE_CMD}" "GetActiveQueue"
    sleep 1

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
        echo "Dispatcher still running after shutdown command" >>${RESFILE}
    else
        echo "Dispatcher has shut down successfully" >>${RESFILE}
        DISPATCHER_RUNNING=0
    fi

    # Compare output to gold standard
    compareToGold ${RESFILE}
    ((TESTCOUNT++))
fi

#------------------------------------------------------------------------------
#  TEST c
#  redo a simulation
#------------------------------------------------------------------------------
TFILES="c"
STEP=0
if [[ "${SINGLETEST}${TFILES}" = "${TFILES}" || "${SINGLETEST}${TFILES}" = "${TFILES}${TFILES}" ]]; then
    setupData
    startDispatcher
    echo -n "Test ${TFILES} - Redo a simulation... "
    #----------------------------------
    # Redo simulation 5
    #----------------------------------
    REDO_CMD='{"command":"Redo","username":"test-user","data":{"SID":5,"MachineID":"test-machine"}}'
    # echo "Sending Book command: ${REDO_CMD}"
    RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" -d "${REDO_CMD}" http://localhost:8250/command)
    sleep 1 # Ensure the response is fully captured
    RESFILE="${TFILES}${STEP}"
    echo "${RESPONSE}" >"${RESFILE}"
    compareToGold "${RESFILE}"
    ((TESTCOUNT++))
fi

echo "Total tests: ${TESTCOUNT}"
echo "Total errors: ${ERRORCOUNT}"
if [ "${ERRORCOUNT}" -gt 0 ]; then
    exit 2
fi
