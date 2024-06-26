#!/bin/bash

# URL of the dispatcher
DISPATCHER_URL="http://localhost:8250/command"

# Function to generate a random string
generate_random_string() {
  local LENGTH=$1
  echo $(LC_ALL=C tr -dc 'a-zA-Z0-9' </dev/urandom | head -c "$LENGTH")
}

# Function to add a random simulation to the queue
add_random_simulation() {
  local FILENAME=$(generate_random_string 8)
  local NAME="Simulation_$(generate_random_string 5)"
  local PRIORITY=$((RANDOM % 10 + 1))
  local DESCRIPTION="Random description $(generate_random_string 10)"
  local URL="http://localhost:8080"

  local REQUEST_DATA=$(cat <<EOF
{
  "command": "NewSimulation",
  "username": "test-user",
  "data": {
    "file": "${FILENAME}.json",
    "name": "${NAME}",
    "priority": ${PRIORITY},
    "description": "${DESCRIPTION}",
    "url": "${URL}"
  }
}
EOF
  )

  curl -s -X POST "${DISPATCHER_URL}" -d "${REQUEST_DATA}" -H "Content-Type: application/json" > /dev/null
  echo "Added simulation: ${NAME} with priority ${PRIORITY}"
}

# Add 10 random simulations
for i in {1..10}; do
  add_random_simulation
done

echo "Finished adding 10 random simulations."
