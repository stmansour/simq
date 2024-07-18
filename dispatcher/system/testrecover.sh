#!/bin/bash

# Recovery scenarios
#
# 1. SID 1 is Booked but /var/lib/simd/simulations/1 does not exist. The dispatcher's copy of the
#    config file does exist in /var/lib/dispatcher/1/fast.json5
#    Recovery: simd rebooks SID 1
# 2. SID 2 is Booked but /var/lib/simd/simulations/1/med.json5 (the config file) does not exist.
#    The config file exists in dispatchers /var/lib/dispatcher/2/fast.json5 .
#    Recovery: simd rebooks SID 2
#----------------------------------------------------------------------------------------------------

PLATOMACHINEID="7cf2ec5736624ae680e87e3587c5faec"

./shutdown.sh
./setup.sh

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

