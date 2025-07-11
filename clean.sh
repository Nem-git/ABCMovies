#!/bin/sh


# PHP
cd ./src/backend/
npx prettier --write ./public/
npx prettier --write ./src/
cd ../../

./src/backend/vendor/bin/phpcbf ./src/backend/public
./src/backend/vendor/bin/phpcbf ./src/backend/src

./src/backend/vendor/bin/php-cs-fixer fix ./src/backend/public
./src/backend/vendor/bin/php-cs-fixer fix ./src/backend/src

# Python
./src/python-backend/.venv/bin/black -t py313 ./src/python-backend/main.py
./src/python-backend/.venv/bin/black -t py313 ./src/python-backend/app

# Svelte
cd ./src/frontend/src/
npx prettier --write .
cd ../../../
