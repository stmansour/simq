#!/bin/bash
#
#  Steve's MacBook Pro:  CCE57473-4791-5E21-977E-F7E2B9145337
#  Plato server:         7cf2ec5736624ae680e87e3587c5faec

MYSQL=$(which mysql)
echo "MYSQL = ${MYSQL}"

SIMDDIR=$(grep "SimdSimulationsDir" simdconf.json5| sed 's/".*"://' | sed 's/[", ]//g')
SIMDHOME=$(grep "SimdHome" simdconf.json5| sed 's/".*"://' | sed 's/[", ]//g')
RESULTSDIR=$(grep "SimResultsDir" simdconf.json5| sed 's/".*"://' | sed 's/[", ]//g')
DISPDIR=$(grep "DispatcherQueueDir" simdconf.json5| sed 's/".*"://' | sed 's/[", ]//g')
echo "SIMDDIR = ${SIMDDIR}"
echo "RESULTSDIR = ${RESULTSDIR}"
echo "DISPDIR = ${DISPDIR}"

YEAR=$(date +"%Y")
MONTH=$(date +"%-m")
DAY=$(date +"%-d")


echo "Year: $YEAR"
echo "Month: $MONTH"
echo "Day: $DAY"

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
#
#------------------------------------------------------------------------------
startSimd() {
    killall simd >/dev/null
    ./simd &
    SIMD_PID=$!
    echo "simd started, SIMD_PID = ${SIMD_PID}"

}

# Useful commands
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
    echo "test a"
    cleanDirectories
    tar xvf "testdata/${TFILES}/disp_1.tar" -C "${DISPDIR}"
    tar xvf "testdata/${TFILES}/simd_1.tar" -C "${SIMDDIR}"
    ${MYSQL} simqtest < "testdata/${TFILES}/simqtest_1.sql"

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
