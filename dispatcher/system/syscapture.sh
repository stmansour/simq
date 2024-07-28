#!/bin/bash
CWD=$(pwd)
#-------------------------------------
#  capture dispatcher's data...
#-------------------------------------
cd /var/lib/dispatcher/
tar cvf disp.tar qdconfigs
mv disp.tar "${CWD}"

#-------------------------------------
#  capture simd's data...
#-------------------------------------
cd /var/lib/simd/
tar cvf simd.tar simulations
mv simd.tar "${CWD}"

#-------------------------------------
#  capture simq database...
#-------------------------------------
mysqldump simqtest >simq.sql


