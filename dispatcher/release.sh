#!/bin/bash

#-----------------------------------
# Shut down the dispatcher service
#-----------------------------------
sudo systemctl stop plato.dispatcher.service

#-----------------------------------------------------------
# Copy the updated dispatcher files to the release location
#-----------------------------------------------------------
cp dispatcher /usr/local/simq/dispatcher/

#-----------------------------------
# Start the dispatcher service
#-----------------------------------
sudo systemctl start plato.dispatcher.service

