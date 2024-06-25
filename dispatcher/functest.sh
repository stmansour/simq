#!/bin/bash

RUNSINGLETEST=0
TESTCOUNT=0
ERRORCOUNT=0
ARCHIVE=arch

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
        -e 's/^Version:.*/Version: VERSION_PLACEHOLDER/' \
        -e 's/^Available cores:.*/Version: PLACEHOLDER/' \
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

#------------------------------------------------------------------------------
#  TEST a
#  initial dispatcher test
#------------------------------------------------------------------------------
TFILES="a"
STEP=0
if [[ "${SINGLETEST}${TFILES}" = "${TFILES}" || "${SINGLETEST}${TFILES}" = "${TFILES}${TFILES}" ]]; then
    echo -n "Test ${TFILES} - "
    echo -n "Basic dispatcher test... "
    RESFILE="${TFILES}${STEP}"
    echo "BASIC dispatcher test..." > ${RESFILE}

    #-------------------------------------------------------
    # start a new dispatcher with a clean database table
    #-------------------------------------------------------
    echo "DROP TABLE IF EXISTS Queue;" | /usr/local/mysql/bin/mysql simq
    killall dispatcher
    SERVER="http://localhost:8250"
    ./dispatcher >DISPATCHER.log 2>&1 &
    pgrep dispatcher >> ${RESFILE} 2>&1
    sleep 1 # Wait for the server to start

    #-------------------------------------------------------
    # TEST COMMAND - NewSimulation
    #-------------------------------------------------------
    DISPATCHER_RESPONSE=$(curl -s -X POST "${SERVER}"/command -d '{ "command": "NewSimulation", "username": "testuser",
        "data": { "file": "/path/to/simulation.tar.gz", "name": "Test Simulation", "priority": 5, "description": "A test simulation", "url": "http://localhost:8080" }
        }' -H "Content-Type: application/json") >> ${RESFILE} 2>&1
    echo "DISPATCHER_RESPONSE: ${DISPATCHER_RESPONSE}"  >> ${RESFILE} 2>&1
    SID=$(echo "${DISPATCHER_RESPONSE}" | grep -o '[0-9]\+')  >> ${RESFILE} 2>&1
    echo "Added simulation with SID: ${SID}"  >> ${RESFILE} 2>&1

    # Update the simulation
    DISPATCHER_RESPONSE=$(curl -s -X POST "${SERVER}"/command -d '{ "command": "UpdateItem", "username": "testuser", "data": {"sid": '"${SID}"', "priority": 10, "description": "Updated description"} }' -H "Content-Type: application/json")  >> ${RESFILE} 2>&1
    echo "DISPATCHER_RESPONSE: ${DISPATCHER_RESPONSE}"  >> ${RESFILE} 2>&1
    echo "Updated simulation with SID: ${SID}" >> ${RESFILE} 2>&1

    # Request the active queue
    DISPATCHER_RESPONSE=$(curl -s -X POST "${SERVER}"/command -d '{ "command": "GetActiveQueue", "username": "testuser" }' -H "Content-Type: application/json")  >> ${RESFILE} 2>&1
    echo "DISPATCHER_RESPONSE: ${DISPATCHER_RESPONSE}"  >> ${RESFILE} 2>&1

    echo "Active Queue:"  >> ${RESFILE} 2>&1
    echo "${DISPATCHER_RESPONSE}" | jq  >> ${RESFILE} 2>&1

    # Delete the simulation
    DISPATCHER_RESPONSE=$(curl -s -X POST "${SERVER}"/command -d '{ "command": "DeleteItem", "username": "testuser", "data": { "sid": '"${SID}"' } }' -H "Content-Type: application/json")  >> ${RESFILE} 2>&1
    echo "DISPATCHER_RESPONSE: ${DISPATCHER_RESPONSE}"  >> ${RESFILE} 2>&1

    # Shutdown the server
    DISPATCHER_RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" -d '{"command": "Shutdown", "username": "test_user"}' "${server}"_url)  >> ${RESFILE} 2>&1
    echo "DISPATCHER_RESPONSE: ${DISPATCHER_RESPONSE}"  >> ${RESFILE} 2>&1

    #compareToGold ${RESFILE}
    ((TESTCOUNT++))
fi

#------------------------------------------------------------------------------
#  TEST b
#  test linguistic influencer
#------------------------------------------------------------------------------
TFILES="b"
STEP=0

echo "Total tests: ${TESTCOUNT}"
echo "Total errors: ${ERRORCOUNT}"
if [ "${ERRORCOUNT}" -gt 0 ]; then
    exit 2
fi
