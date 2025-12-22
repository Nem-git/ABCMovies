#!/bin/sh

# Svelte
cd ./src/frontend/
npx prettier --write .
npx eslint src/
cd ../../


# Go
gofmt -w ./src/backend/

