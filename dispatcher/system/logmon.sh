#!/bin/bash

LOG_FILE="/usr/local/simq/dispatcher/dispatcher.log"
LOG_DIR=$(dirname "${LOG_FILE}")
LOG_FILENAME=$(basename "${LOG_FILE}")
TAILPID=0

# Function to set the terminal window title
setWindowTitle() {
    local title="$1"
    echo -ne "\033]0;$title\007"
}

# Determine the OS and set the file watcher command
OS=$(uname -s)
if [[ "$OS" == "Linux" ]]; then
    FILE_WATCHER="inotifywait"
    FILE_WATCHER_OPTS="-e create -e delete -q --include $LOG_FILENAME $LOG_DIR"
elif [[ "$OS" == "Darwin" ]]; then
    FILE_WATCHER="fswatch"
    FILE_WATCHER_OPTS="-0 $LOG_DIR"
else
    echo "Unsupported OS: $OS"
    exit 1
fi

# Check if the appropriate file watcher tool is installed
if ! command -v ${FILE_WATCHER} &> /dev/null; then
    if [[ "${FILE_WATCHER}" == "inotifywait" ]]; then
        echo "inotifywait not found. Please install the inotify-tools package."
    else
        echo "fswatch not found. Please install it using brew: brew install fswatch"
    fi
    exit 1
fi

# Function to handle log file processing
processLogfile() {
  tail -f "${LOG_FILE}" > /dev/stdout &
  TAILPID=$!
  echo "**** TAIL PID = ${TAILPID} ****"
}

awaitCreation() {
    echo "**** AWAITING CREATION ****"
    if [[ "${FILE_WATCHER}" == "inotifywait" ]]; then
        while true; do
            inotifywait "${FILE_WATCHER_OPTS}"
            if [ $? -eq 0 ]; then
                echo "**** $LOG_FILENAME CREATED ****"
                break
            fi
        done
    else
        while true; do
            fswatch "${FILE_WATCHER_OPTS}" | while read -dr "" event; do
                if [[ "$event" == "${LOG_FILE}" ]]; then
                    echo "**** $LOG_FILENAME CREATED ****"
                    break 2
                fi
            done
        done
    fi
}

awaitDeletion() {
    echo "**** AWAITING DELETION ****"
    if [[ "${FILE_WATCHER}" == "inotifywait" ]]; then
        while true; do
            inotifywait "${FILE_WATCHER_OPTS}"
            if [ $? -eq 0 ]; then
                echo "**** $LOG_FILENAME DELETED ****"
                break
            fi
        done
    else
        while true; do
            fswatch "${FILE_WATCHER_OPTS}" | while read -dr "" event; do
                if [[ "$event" == "${LOG_FILE}" ]]; then
                    echo "**** $LOG_FILENAME DELETED ****"
                    break 2
                fi
            done
        done
    fi
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

LOG_DIR=$(dirname "${LOG_FILE}")
LOG_FILENAME=$(basename "${LOG_FILE}")

echo "Monitoring log file: ${LOG_FILE}"

# Main loop to monitor the log file
while true; do
  #--------------------------------------------------------------------
  # If the logfile exists, tail it. If not, wait for it to be created
  #--------------------------------------------------------------------
  if [ -f "${LOG_FILE}" ] && [ "$TAILPID" = "0" ]; then
    processLogfile
    awaitDeletion
    if [ "${TAILPID}" != "0" ]; then
        echo "**** KILL TAIL PROCESS ${TAILPID} ****"
        kill "${TAILPID}"
        TAILPID=0
      fi
  fi
  awaitCreation
done
