#!/bin/bash
SAVEDIR=$(pwd)
echo "Starting dispatcher"
cd /usr/local/simq/dispatcher
nohup ./dispatcher &

sleep 1

echo "Starting simd"
cd /usr/local/simq/simd
nohup ./simd &

echo "PIDS"
pgrep dispatcher
pgrep simd

# Array to store filenames (modify this with your actual filenames)
filenames=(
  "fast.json5"
  "med.json5"
  "long.json5"
  "fast.json5"
  "med.json5"
  "long.json5"
  "fast.json5"
  "med.json5"
  "long.json5"
  # Add more filenames here
)

echo "wait 5 seconds for all the processes to get up and running..."
sleep 5

echo "Now, add some workload"
cd "${SAVEDIR}"
# Loop through each filename
for f in "${filenames[@]}"; do
    psq -action add -file "$f"
done

psq -action list

echo "Monitor dispatcher:    tail -f /usr/local/simq/dispatcher/dispatcher.log"
echo "Monitor simd:          tail -f /usr/local/simq/simd/simd.log"
