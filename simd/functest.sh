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
startDispatcher() {
    if ((DISPATCHER_RUNNING == 0)); then
        killall -9 dispatcher >/dev/null 2>&1

        #-------------------------------------------------------
        # start a new dispatcher with a clean database table
        #-------------------------------------------------------
        echo "DROP TABLE IF EXISTS Queue;" | ${MYSQL} simqtest
        rm -rf qdconfigs

        ./dispatcher >DISPATCHER.log 2>&1 &
        DISPATCHER_PID=$!
        sleep 2
        DISPATCHER_RUNNING=1
    fi
}

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

    while [ $seconds -lt $timeout ]; do
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
#  loadDataset - load the dataset into the database
#  INPUTS
#    $1 = dataset number
#------------------------------------------------------------------------------
loadDataset() {
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
        ASKBEFOREEXIT=1
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

#------------------------------------------------------------------------------
#  TEST a
#  initial dispatcher test - a simulation was booked, but no simulation
#  directory exists
#------------------------------------------------------------------------------
TFILES="a"
STEP=0
if [[ "${SINGLETEST}${TFILES}" = "${TFILES}" || "${SINGLETEST}${TFILES}" = "${TFILES}${TFILES}" ]]; then
    echo "test ${TFILES} - individual test recover booked simulation, no simulation directory"
    cleanDirectories
    loadDataset 1

    TARGETFILE="${RESULTSDIR}/${YEAR}/${MONTH}/${DAY}/1/finrep.csv"
    echo "Waiting for the creation of: ${TARGETFILE}"
    startSimd
    TIMELIMIT=20
    usefulCommands
    if checkFileExists "${TARGETFILE}" "${TIMELIMIT}"; then
        echo "PASS"
    else
        echo "FAIL... ${TARGETFILE} was not present after ${TIMELIMIT} sec"
    fi
    
    kill $SIMD_PID >/dev/null 2>&1
fi

#------------------------------------------------------------------------------
#  TEST b
#------------------------------------------------------------------------------
TFILES="b"
STEP=0
if [[ "${SINGLETEST}${TFILES}" = "${TFILES}" || "${SINGLETEST}${TFILES}" = "${TFILES}${TFILES}" ]]; then
    echo "test ${TFILES} - test recover booked simulation, simulation directory exists, but no config file"
    cleanDirectories
    loadDataset 2

    TARGETFILE="${RESULTSDIR}/${YEAR}/${MONTH}/${DAY}/2/finrep.csv"
    echo "Waiting for the creation of: ${TARGETFILE}"
    startSimd
    TIMELIMIT=20
    usefulCommands
    if checkFileExists "${TARGETFILE}" "${TIMELIMIT}"; then
        echo "PASS"
    else
        echo "FAIL... ${TARGETFILE} was not present after ${TIMELIMIT} sec"
    fi
    
    kill $SIMD_PID >/dev/null 2>&1
fi

#------------------------------------------------------------------------------
#  TEST c
#------------------------------------------------------------------------------
TFILES="c"
STEP=0
if [[ "${SINGLETEST}${TFILES}" = "${TFILES}" || "${SINGLETEST}${TFILES}" = "${TFILES}${TFILES}" ]]; then
    echo "test ${TFILES} - test recover booked simulation, simulation directory exists, config file exists"
    cleanDirectories
    loadDataset 3

    TARGETFILE="${RESULTSDIR}/${YEAR}/${MONTH}/${DAY}/3/finrep.csv"
    echo "Waiting for the creation of: ${TARGETFILE}"
    startSimd
    TIMELIMIT=20
    usefulCommands
    if checkFileExists "${TARGETFILE}" "${TIMELIMIT}"; then
        echo "PASS"
    else
        echo "FAIL... ${TARGETFILE} was not present after ${TIMELIMIT} sec"
    fi
    
    kill $SIMD_PID >/dev/null 2>&1
fi

#------------------------------------------------------------------------------
#  TEST d
#------------------------------------------------------------------------------
TFILES="d"
STEP=0
if [[ "${SINGLETEST}${TFILES}" = "${TFILES}" || "${SINGLETEST}${TFILES}" = "${TFILES}${TFILES}" ]]; then
    echo "test ${TFILES} - test recover booked simulation, simulation directory exists, results completed"
    cleanDirectories
    loadDataset 4

    TARGETFILE="${RESULTSDIR}/${YEAR}/${MONTH}/${DAY}/4/finrep.csv"
    echo "Waiting for the creation of: ${TARGETFILE}"
    startSimd
    TIMELIMIT=20
    usefulCommands
    if checkFileExists "${TARGETFILE}" "${TIMELIMIT}"; then
        echo "PASS"
    else
        echo "FAIL... ${TARGETFILE} was not present after ${TIMELIMIT} sec"
    fi
    
    kill $SIMD_PID >/dev/null 2>&1
fi

#------------------------------------------------------------------------------
#  TEST e
#------------------------------------------------------------------------------
TFILES="e"
STEP=0
if [[ "${SINGLETEST}${TFILES}" = "${TFILES}" || "${SINGLETEST}${TFILES}" = "${TFILES}${TFILES}" ]]; then
    echo "test ${TFILES} - test recover booked simulation, simulation directory exists, results.tar.gz exists"
    cleanDirectories
    loadDataset 5

    TARGETFILE="${RESULTSDIR}/${YEAR}/${MONTH}/${DAY}/5/finrep.csv"
    echo "Waiting for the creation of: ${TARGETFILE}"
    startSimd
    TIMELIMIT=20
    usefulCommands
    if checkFileExists "${TARGETFILE}" "${TIMELIMIT}"; then
        echo "PASS"
    else
        echo "FAIL... ${TARGETFILE} was not present after ${TIMELIMIT} sec"
    fi
    
    kill $SIMD_PID >/dev/null 2>&1
fi
