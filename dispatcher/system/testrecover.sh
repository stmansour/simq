#!/bin/bash

# Recovery scenarios
#
# 1. SID 1 is Booked but /var/lib/simd/simulations/1 does not exist. The dispatcher's copy of the
#    config file does exist in /var/lib/dispatcher/1/fast.json5
#    Recovery: simd rebooks SID 1
# 2. SID 2 is Booked but /var/lib/simd/simulations/1/med.json5 (the config file) does not exist.
#    The config file exists in dispatchers /var/lib/dispatcher/2/fast.json5 .
#    Recovery: simd rebooks SID 2
# 3. SID 3 a running simulator working on sid 3.  We'll set to a med.json5 config 
#----------------------------------------------------------------------------------------------------

PLATOMACHINEID="7cf2ec5736624ae680e87e3587c5faec"

./shutdown.sh
./clean.sh
./setup.sh

#---------------------------------------------------------------------
# setup.sh includes the recovery where we have a running simulator.
# Start that simulator...
#---------------------------------------------------------------------
CWD=$(pwd)
cd /var/lib/simd/simulations/3
/usr/local/plato/bin/simulator -c med.json5 -SID 3 -DISPATCHER http://192.168.5.100:8250/ >sim.log 2>&1 &
cd "${CWD}"

echo "Starting DISPATCHER..."
SAVEDIR=$(pwd)
echo "Starting dispatcher"
cd /usr/local/simq/dispatcher
nohup ./dispatcher &

sleep 1

echo "Starting SIMD"
cd /usr/local/simq/simd
nohup ./simd &


cat <<EOF
HANDY COMMANDS
==============
cd /usr/local/simq/dispatcher ; logmon disp
cd /usr/local/simq/simd ; logmon simd
tree /var/lib/dispatcher/qdconfigs
tree /var/lib/simd/simulations
tree /genome/simres/2024/
EOF

