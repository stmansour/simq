#!/bin/bash
./shutdown.sh
./start.sh

echo "wait 5 seconds for all the processes to get up and running..."
sleep 5

./addload.sh
