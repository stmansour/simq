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

