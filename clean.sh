#!/bin/sh


# PHP
# cd ./src/backend/
# npx prettier --write ./public/
# npx prettier --write ./src/
# cd ../../

# ./src/backend/vendor/bin/phpcbf ./src/backend/public
# ./src/backend/vendor/bin/phpcbf ./src/backend/src

# ./src/backend/vendor/bin/php-cs-fixer fix ./src/backend/public
# ./src/backend/vendor/bin/php-cs-fixer fix ./src/backend/src

# # Python
# ./src/python-backend/.venv/bin/black -t py313 ./src/python-backend/src

# Svelte
cd ./src/frontend/
npx prettier --write .
npx eslint src/
cd ../../


# Go
gofmt -w ./src/go-backend/

