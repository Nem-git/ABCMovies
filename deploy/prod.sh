#!/bin/bash

ROOT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && cd .. && pwd)
docker compose -f $ROOT_DIR/compose.yaml --profile prod up --build --force-recreate -d
