#/bin/bash
tar xvf disp.tar -C /var/lib/dispatcher/
tar xvf simd.tar -C /var/lib/simd/

MYSQL=$(which mysql)
echo "MYSQL = ${MYSQL}"
${MYSQL} simqtest < simq.sql
