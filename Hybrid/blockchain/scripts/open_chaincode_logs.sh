#!/bin/bash

docker ps -a --format '{{.ID}} {{.Image}}' | grep 'dev-' | awk '{print $1}'

docker logs -f $(docker ps -a --format '{{.ID}} {{.Image}}' | grep 'dev-' | awk '{print $1}' | head -n 1)

