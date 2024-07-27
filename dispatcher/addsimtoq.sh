#!/bin/bash
curl -v -X POST http://localhost:8250/command \
    -F "command=NewSimulation" \
    -F "username=test-user" \
    -F "data={\"name\":\"Test Simulation\",\"priority\":5,\"description\":\"A test simulation\",\"url\":\"http://localhost:8080\",\"OriginalFilename\":\"config.json5\"}" \
    -F "file=@config.json5"

