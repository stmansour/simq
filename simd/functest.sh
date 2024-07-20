#!/bin/bash
#
#  Steve's MacBook Pro:  CCE57473-4791-5E21-977E-F7E2B9145337
#  Plato server:         7cf2ec5736624ae680e87e3587c5faec

MYSQL=$(which mysql)

SIMDDIR=$(grep "SimdSimulationsDir" simdconf.json5| sed 's/".*"://' | sed 's/[", ]//g')
SIMDHOME=$(grep "SimdHome" simdconf.json5| sed 's/".*"://' | sed 's/[", ]//g')
RESULTSDIR=$(grep "SimResultsDir" simdconf.json5| sed 's/".*"://' | sed 's/[", ]//g')
DISPDIR=$(grep "DispatcherQueueDir" simdconf.json5| sed 's/".*"://' | sed 's/[", ]//g')
YEAR=$(date +"%Y")
MONTH=$(date +"%-m")
DAY=$(date +"%-d")
TESTCOUNT=0
ERRORCOUNT=0
START_TIME=$(date +%s)

ShowDuration() {
    END_TIME=$(date +%s)
    echo "End time: ${END_TIME}"
    ELAPSED_TIME=$((END_TIME - START_TIME))
    HOURS=$((ELAPSED_TIME / 3600))
    MINUTES=$(((ELAPSED_TIME % 3600) / 60))
    SECONDS=$((ELAPSED_TIME % 60))
    echo "Elapsed time: ${HOURS}h ${MINUTES}m ${SECONDS}s"
}

#--------------------------------------------------------------------------------------
# cleanDirectories removes all data files maintained by simd and dispatcher,
# including the simulation results directory. This is so we start with a clean-slate. 
#--------------------------------------------------------------------------------------
cleanDirectories() {
    rm -rf "${SIMDDIR:?}/"*
    rm -rf "${RESULTSDIR:?}/"*
    rm -rf "${DISPDIR:?}/"*
}

#------------------------------------------------------------------------------
# startDispatcher - kill any existing dispatcher, then start the dispatcher.
# INPUTS
#    none yet
#------------------------------------------------------------------------------
# startDispatcher() {
#     if ((DISPATCHER_RUNNING == 0)); then
#         killall -9 dispatcher >/dev/null 2>&1

#         #-------------------------------------------------------
#         # start a new dispatcher with a clean database table
#         #-------------------------------------------------------
#         echo "DROP TABLE IF EXISTS Queue;" | ${MYSQL} simqtest
#         rm -rf qdconfigs

#         ./dispatcher >DISPATCHER.log 2>&1 &
#         DISPATCHER_PID=$!
#         sleep 2
#         DISPATCHER_RUNNING=1
#     fi
# }

#------------------------------------------------------------------------------
# checkFileExists - wait, for up to 10 seconds, for a file to 
# INPUTS
#    $1 = file to check for
#    $2 = amount of time in seconds we're willing to wait
#------------------------------------------------------------------------------
checkFileExists() {
    local file="$1"
    local timeout="$2"
    local interval=1  # Check every 1 second
    local seconds=0

    echo "checkFileExists looking for: ${file}"

    while (( seconds < timeout )); do
        if [ -f "$file" ]; then
            echo "File $file found after $seconds seconds."
            return 0
        fi
        sleep $interval
        seconds=$((seconds + interval))
    done

    return 1
}

#------------------------------------------------------------------------------
#  startSimd - kill any existing simd, then start the simd.
#------------------------------------------------------------------------------
startSimd() {
    killall simd >/dev/null
    ./simd &
    SIMD_PID=$!
    echo "simd started, SIMD_PID = ${SIMD_PID}"
}

#------------------------------------------------------------------------------
#  startDispatcher - kill any existing dispatcher, then start the dispatcher.
#------------------------------------------------------------------------------
startDispatcher() {
    killall dispatcher >/dev/null
    CWD=$(pwd)
    cd ../dispatcher || exit 2
    ./dispatcher &
    DISPATCHER_PID=$(pgrep dispatcher)
    cd "${CWD}" || exit 2
    echo "dispatcher started, DISPATCHER_PID = ${DISPATCHER_PID}"
}

shutdownDispatcher() {
    kill -9 "${DISPATCHER_PID}"
}

#------------------------------------------------------------------------------
#  loadDataset - load the dataset into the database
#  INPUTS
#    $1 = dataset number
#------------------------------------------------------------------------------
loadDataset() {
    cleanDirectories
    tar xvf "testdata/${1}/disp.tar" -C "${DISPDIR}"
    tar xvf "testdata/${1}/simd.tar" -C "${SIMDDIR}"
    ${MYSQL} simqtest < "testdata/${1}/simq.sql"
}

#------------------------------------------------------------------------------
# Useful commands
#------------------------------------------------------------------------------
usefulCommands() {
cat <<EOF
Useful commands:
    tail -f ${SIMDHOME}/simd.log
    tree ${SIMDDIR}
    tree ${RESULTSDIR}
EOF
}

#------------------------------------------------------------------------------
# checkResults looks for TARGETFILE to appear within TIMELIMIT. 
# If it does, it passes. If not, it fails and ERRORCOUNT is incremented
# TESTCOUNT is incremented by this function no matter what the results are.
#------------------------------------------------------------------------------
checkResults() {
    ((TESTCOUNT++))
    if checkFileExists "${TARGETFILE}" "${TIMELIMIT}"; then
        echo "*** PASS ***"
    else
        echo "FAIL... ${TARGETFILE} was not present after ${TIMELIMIT} sec"
        ((ERRORCOUNT++))
    fi
    
    kill $SIMD_PID >/dev/null 2>&1
}

#------------------------------------------------------------------------------
# delayForSimulatorCheck = For this test, we want to see if simd can connect
# with a running simulator. So we delay a bit to give simd time to do the
# check and connect with it.  We wait 4 seconds, which should be plenty of
# time.  Then we'll send the simulator a command to end what it's doing and
# make sure that simd sees that it's done and sends the results to dispatcher.
#------------------------------------------------------------------------------
delayForSimulatorCheck() {
    echo "Delaying for 4 seconds to let simd connect with the simulator..."
    sleep 4
    echo "Sending end command to simulator to end after next gen..."
    curl http://localhost:8090/stopsim
    checkResults
}

###############################################################################
#    INPUT
###############################################################################
HOSTNAME=$(hostname)
if [ "${HOSTNAME}" != "StevesMcBookPro.attlocal.net" ]; then
    echo "This script kills simd, resets the simq database, and many other similarly"
    echo "destructive things. It should only be run on StevesMcBookPro.attlocal.net"
    exit 1
fi

while getopts "acd:t:" o; do
    echo "o = ${o}"
    case "${o}" in
    a)
        # ASKBEFOREEXIT=1
        echo "WILL ASK BEFORE EXITING ON ERROR"
        ;;

    c)  echo -n "CLEANING DIRECTORIES..."
        cleanDirectories
        echo "DONE"
        exit 0
        ;;

    d)  DATASET="${OPTARG}"
        echo "CLEANING DIRECTORIES..."
        cleanDirectories
        echo "LOADING DATASET ${DATASET}..."
        loadDataset "${DATASET}"
        echo "DONE"
        exit 0
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

startDispatcher
sleep 1

#------------------------------------------------------------------------------
#  TEST a
#  initial dispatcher test - a simulation was booked, but no simulation
#  directory exists
#------------------------------------------------------------------------------
TFILES="a"
# STEP=0
if [[ "${SINGLETEST}${TFILES}" = "${TFILES}" || "${SINGLETEST}${TFILES}" = "${TFILES}${TFILES}" ]]; then
    echo "test ${TFILES} - individual test recover booked simulation, no simulation directory"
    loadDataset 1

    TARGETFILE="${RESULTSDIR}/${YEAR}/${MONTH}/${DAY}/1/finrep.csv"
    echo "Waiting for the creation of: ${TARGETFILE}"
    startSimd
    TIMELIMIT=20
    checkResults
fi

#------------------------------------------------------------------------------
#  TEST b
#------------------------------------------------------------------------------
TFILES="b"
# STEP=0
if [[ "${SINGLETEST}${TFILES}" = "${TFILES}" || "${SINGLETEST}${TFILES}" = "${TFILES}${TFILES}" ]]; then
    echo "test ${TFILES} - test recover booked simulation, simulation directory exists, but no config file"
    loadDataset 2
    TARGETFILE="${RESULTSDIR}/${YEAR}/${MONTH}/${DAY}/2/finrep.csv"
    echo "Waiting for the creation of: ${TARGETFILE}"
    startSimd
    TIMELIMIT=20
    usefulCommands
    checkResults
fi

#------------------------------------------------------------------------------
#  TEST c
#------------------------------------------------------------------------------
TFILES="c"
# STEP=0
if [[ "${SINGLETEST}${TFILES}" = "${TFILES}" || "${SINGLETEST}${TFILES}" = "${TFILES}${TFILES}" ]]; then
    echo "test ${TFILES} - test recover booked simulation, simulation directory exists, config file exists"
    loadDataset 3
    TARGETFILE="${RESULTSDIR}/${YEAR}/${MONTH}/${DAY}/3/finrep.csv"
    echo "Waiting for the creation of: ${TARGETFILE}"
    startSimd
    TIMELIMIT=20
    usefulCommands
    checkResults
fi

#------------------------------------------------------------------------------
#  TEST d
#------------------------------------------------------------------------------
TFILES="d"
# STEP=0
if [[ "${SINGLETEST}${TFILES}" = "${TFILES}" || "${SINGLETEST}${TFILES}" = "${TFILES}${TFILES}" ]]; then
    echo "test ${TFILES} - test recover booked simulation, simulation directory exists, results completed"
    loadDataset 4
    TARGETFILE="${RESULTSDIR}/${YEAR}/${MONTH}/${DAY}/4/finrep.csv"
    echo "Waiting for the creation of: ${TARGETFILE}"
    startSimd
    TIMELIMIT=10
    usefulCommands
    checkResults
fi

#------------------------------------------------------------------------------
#  TEST e
#------------------------------------------------------------------------------
TFILES="e"
# STEP=0
if [[ "${SINGLETEST}${TFILES}" = "${TFILES}" || "${SINGLETEST}${TFILES}" = "${TFILES}${TFILES}" ]]; then
    echo "test ${TFILES} - test recover booked simulation, simulation directory exists, results.tar.gz exists."
    loadDataset 5
    TARGETFILE="${RESULTSDIR}/${YEAR}/${MONTH}/${DAY}/5/finrep.csv"
    echo "Waiting for the creation of: ${TARGETFILE}"
    startSimd
    TIMELIMIT=10
    usefulCommands
    checkResults
fi

#------------------------------------------------------------------------------
#  TEST f
#------------------------------------------------------------------------------
TFILES="f"
# STEP=0
if [[ "${SINGLETEST}${TFILES}" = "${TFILES}" || "${SINGLETEST}${TFILES}" = "${TFILES}${TFILES}" ]]; then
    echo "test ${TFILES} - test recover booked simulation, and it has a running simulator working on it."
    loadDataset 6
    TARGETFILE="${RESULTSDIR}/${YEAR}/${MONTH}/${DAY}/6/finrep.csv"
    killall simulator
    CWD=$(pwd)
    cd /var/lib/simd/simulations/6 || exit 2; /usr/local/plato/bin/simulator -c med.json5 -SID 6 -DISPATCHER http://localhost:8250/ >sim.log 2>&1 &
    cd "${CWD}" || exit 2
    startSimd
    TIMELIMIT=120
    usefulCommands
    echo "Waiting for the creation of: ${TARGETFILE}"
    delayForSimulatorCheck
fi

shutdownDispatcher
sleep 1

echo "------------------------------------------------------------------"
ShowDuration
echo "Total tests: ${TESTCOUNT}"
echo "Total errors: ${ERRORCOUNT}"
if [ "${ERRORCOUNT}" -gt 0 ]; then
    exit 2
fi
if [ "${ERRORCOUNT}" -eq 0 ]; then
    echo "****************************"
    echo "***   ALL TESTS PASSED   ***"
    echo "****************************"
fi