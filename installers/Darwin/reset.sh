#!/bin/bash
# ONLY DO THIS ON THE DEVELOPMENT MACHINE, NEVER ON THE SERVER
#
# Also, you can run the .pkg with the following command:
#
# sudo installer -pkg /Users/stevemansour/Documents/src/go/src/simq/installers/mac/simq.pkg -target /

# This will need to run as root
rm -rf /var/lib/simd
rm -rf /usr/local/simq

# delete the users too...
dscl . -delete /Users/simd
dscl . -delete /Groups/simd
