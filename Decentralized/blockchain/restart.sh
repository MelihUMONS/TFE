#!/bin/bash

./network.sh down
./scripts/delete_all_databases.sh
./network.sh up
sleep 3
./network.sh createChannel
./network.sh deployCC -l javascript






