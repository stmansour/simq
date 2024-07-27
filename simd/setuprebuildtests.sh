#!/bin/bash

MYSQL=$(which mysql)
echo "MYSQL = ${MYSQL}"

SIMULATIONSDIR=$(grep "SimdSimulationsDir" simdconf.json5| sed 's/".*"://' | sed 's/[", ]//g')
RESULTSDIR=$(grep "SimResultsDir" simdconf.json5| sed 's/".*"://' | sed 's/[", ]//g')
QDCONFIGSDIR=$(grep "DispatcherQueueDir" simdconf.json5| sed 's/".*"://' | sed 's/[", ]//g')
# echo "SIMULATIONSDIR = ${SIMULATIONSDIR}, RESULTSDIR = ${RESULTSDIR}, QDCONFIGSDIR = ${QDCONFIGSDIR}"

#---------------------
# ON WITH IT!
#---------------------
echo "+------------------------------------------------------------"
echo "|  Resetting simd's simulations directory"
echo "+------------------------------------------------------------"
rm -rf "${SIMULATIONSDIR:?}/*"
tar xvf sims.tar -C "${SIMULATIONSDIR}/"

echo "+------------------------------------------------------------"
echo "|  Resetting dispatcher queue"
echo "+------------------------------------------------------------"
"${MYSQL}" simqtest < simqtest.sql
echo "SELECT * FROM Queue;" | "${MYSQL}" simqtest

echo "+-----------------------------------------------------------"
echo "|  Resetting /opt/testsimres directory"
echo "+------------------------------------------------------------"
rm -rf "${RESULTSDIR:?}/*"

echo "+------------------------------------------------------------"
echo "|  Resetting dispatcher's qdconfigs directory"
echo "+------------------------------------------------------------"
rm -rf "${QDCONFIGSDIR:?}/*"
tar xvf qds.tar -C "${QDCONFIGSDIR}/"
