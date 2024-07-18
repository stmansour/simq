#!/bin/bash

LOG_FILE="/usr/local/simq/dispatcher/dispatcher.log"
LOG_DIR=$(dirname "$LOG_FILE")
LOG_FILENAME=$(basename "$LOG_FILE")
TAILPID=0

# Function to set the terminal window title
setWindowTitle() {
    local title="$1"
    echo -ne "\033]0;$title\007"
}

# Check if inotifywait is installed
if ! command -v inotifywait &> /dev/null; then
  echo "inotifywait not found. Please install the inotify-tools package."
  exit 1
fi

# Function to handle log file processing
processLogfile() {
  tail -f "$LOG_FILE" > /dev/stdout &
  TAILPID=$!
  echo "**** TAIL PID = ${TAILPID} ****"
}

awaitCreation() {
    echo "**** AWAITING CREATION ****"
    while true; do
        inotifywait -e create -q --include "$LOG_FILENAME" "$LOG_DIR"
        if [ $? -eq 0 ]; then
            echo "**** $LOG_FILENAME CREATED ****"
            break
        fi
    done
}

awaitDeletion() {
    echo "**** AWAITING DELETION ****"
    while true; do
        inotifywait -e delete -q --include "$LOG_FILENAME" "$LOG_DIR"
        if [ $? -eq 0 ]; then
            echo "**** $LOG_FILENAME DELETED ****"
            break
        fi
    done
}

#---------------------------------------------------------------------------------
if [ "$#" -ne 1 ]; then
    echo "Usage: $0 {disp|simd}"
    exit 1
fi

case "$1" in
    disp)
        LOG_FILE="/usr/local/simq/dispatcher/dispatcher.log"  # Replace with the actual path
        setWindowTitle "DISPATCHER LOG"
        ;;
    simd)
        LOG_FILE="/usr/local/simq/simd/simd.log"  # Replace with the actual path
        setWindowTitle "SIMD LOG"
        ;;
    *)
        echo "Invalid option: $1"
        echo "Usage: $0 {disp|simd}"
        exit 1
        ;;
esac

LOG_DIR=$(dirname "$LOG_FILE")
LOG_FILENAME=$(basename "$LOG_FILE")

echo "Monitoring log file: $LOG_FILE"


# Main loop to monitor the log file
while true; do
  #--------------------------------------------------------------------
  # If the logfile exists, tail it. If not, wait for it to be created
  #--------------------------------------------------------------------
  if [ -f "$LOG_FILE" ] && [ ${TAILPID}="0" ]; then
    processLogfile
    awaitDeletion
    if [ "${TAILPID}" != "0" ]; then
        echo "**** KILL TAIL PROCESS ${TAILPID} ****"
        kill "${TAILPID}"
      fi
  fi
  awaitCreation
done

