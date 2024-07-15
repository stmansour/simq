#!/bin/bash

echo "Starting dispatcher"
cd /usr/local/simq/dispatcher
nohup ./dispatcher &

sleep 1

echo "Starting simd"
cd /usr/local/simq/simd
nohup ./simd &

pgrep dispatcher
pgrep simd
