#!/bin/bash

# Check if docker is installed
if ! [ -x "$(command -v docker)" ]; then
          echo 'Unable to find docker command, please install Docker (https://www.docker.com/) and retry' >&2
            exit 1
fi

echo "Removing running functions"
faas-cli rm -f stack.yml

echo "Deleting old templates"
rm -rf template

echo "Pulling latest faas-forward templates"
faas-cli template pull https://github.com/s8sg/faas-forward

echo "Building functions"
faas-cli build -f stack.yml

echo "Deploying functions"
faas-cli deploy -f stack.yml
