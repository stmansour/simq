#!/bin/bash
#
#  THE TEST HERE CAN DO MAJOR DAMAGE BY DELETING IMPORTANT
#  DATA IF IT RUNS ON THE WRONG MACHINES.
#
#  So, we maintain a list of machines on which it's OK to
#  run. If the script finds that it's not on one of these
#  systems it will exit out before doing anything.
#
#  Explanation of this test
#  ------------------------
#  It does the following:
#  1. Resets the dispatcher's qdconfigs/ directory to the contents of qds.tar
#  2. Resets simd's simulations/ directory to the contents of sims.tar
#  3. Resets database simq table Queue to match the contents of dispatcher's qdconfigs/
#  4. should recover all test cases.
#
#  How To Run
#  ----------
#  1. Run this script to set up the environment. It should be run from its source directory.
#  2. In a separate window run dispatcher in its sourcecode home
#  3. Optional: run psq in a separate window to examine the queue
#  4. Optional: run simtalk in a separate window
#  5. start simd:   ./simd
#

ALLOW=0
allowableNames=(
    "StevesMcBookPro.attlocal.net"
    "Steves-2020-Pro.attlocal.net"
)

#-----------------------------------
# Get the current system name
#-----------------------------------
current_hostname=$(hostname)

#---------------------------------------------------------------
# Check the current hostname against each entry in the array
#---------------------------------------------------------------
for name in "${allowableNames[@]}"; do
    if [ "$current_hostname" == "$name" ]; then
        ALLOW=1
        break
    fi
done

if (( ALLOW != 1 )); then
    cat << MEOF
This host is not in the list of hosts where running this script is allowed.
It is possible to lose critical data if it is run on the wrong machine.
If this is an allowable machine, update "allowableName" in this script
and run it again.
MEOF
    exit 1
fi

SIMULATIONSDIR=$(grep "SimdSimulationsDir" simdconf.json5| sed 's/".*"://' | sed 's/[", ]//g')
RESULTSDIR=$(grep "SimResultsDir" simdconf.json5| sed 's/".*"://' | sed 's/[", ]//g')
QDCONFIGSDIR=$(grep "DispatcherQueueDir" simdconf.json5| sed 's/".*"://' | sed 's/[", ]//g')
# echo "SIMULATIONSDIR = ${SIMULATIONSDIR}, RESULTSDIR = ${RESULTSDIR}, QDCONFIGSDIR = ${QDCONFIGSDIR}"

#---------------------
# ON WITH IT!
#---------------------
echo "-------------------------------------------------------------"
echo "|  Resetting simd's simulations directory"
echo "-------------------------------------------------------------"
rm -rf "${SIMULATIONSDIR:?}/*"
tar xvf sims.tar -C "${SIMULATIONSDIR}/"

echo "-------------------------------------------------------------"
echo "|  Resetting dispatcher queue"
echo "-------------------------------------------------------------"
/usr/local/mysql/bin/mysql simqtest < simqtest.sql
echo "SELECT * FROM Queue;" | /usr/local/mysql/bin/mysql simqtest

echo "------------------------------------------------------------"
echo "|  Resetting /opt/TestSimResultsRepo directory"
echo "-------------------------------------------------------------"
rm -rf "${RESULTSDIR:?}/*"

echo "-------------------------------------------------------------"
echo "|  Resetting dispatcher's qdconfigs directory"
echo "-------------------------------------------------------------"
rm -rf "${QDCONFIGSDIR:?}/*"
tar xvf qds.tar -C "${QDCONFIGSDIR}/"
