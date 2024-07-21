#!/bin/bash

#---------------------------------------
# RESET SQL DB
#---------------------------------------
echo "resetting production simq.Queue table"
mysql simq <<EOF
DROP TABLE IF EXISTS Queue;
CREATE TABLE IF NOT EXISTS Queue (
     SID BIGINT AUTO_INCREMENT PRIMARY KEY,
     File VARCHAR(80) NOT NULL,
     Username VARCHAR(40) NOT NULL,
     Name VARCHAR(80) NOT NULL DEFAULT '',
     Priority INT NOT NULL DEFAULT 5,
     Description VARCHAR(256) NOT NULL DEFAULT '',
     MachineID VARCHAR(80) NOT NULL DEFAULT '',
     URL VARCHAR(80) NOT NULL DEFAULT '',
     State INT NOT NULL DEFAULT 0,
     DtEstimate DATETIME,
     DtCompleted DATETIME,
     Created TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
     Modified TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
EOF

#---------------------------------------
# RESET GENOME    (SIMULATION REPOSITORY)
#---------------------------------------
echo "resetting /genome/simres/"
rm -rf /genome/simres/*

#---------------------------------------
# REMOVE DISPATCHER LOGS & TEMP STORAGE
#---------------------------------------
echo "emptying logs and temp storage for dispatcher"
rm -rf /var/lib/dispatcher/qdconfigs
rm -f /usr/local/simq/dispatcher/dispatcher.log

#---------------------------------------
# REMOVE SIMD LOGS & TEMP STORAGE
#---------------------------------------
echo "emptying logs and temp storage for simd"
rm -rf /var/lib/simd/simulations
rm -f /usr/local/simq/simd/simd.log
