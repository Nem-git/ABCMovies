#!/bin/sh

export PHP_CS_FIXER_IGNORE_ENV=true

# PHP
./src/backend/vendor/bin/phpcbf ./src/backend/config
./src/backend/vendor/bin/phpcbf ./src/backend/public
./src/backend/vendor/bin/phpcbf ./src/backend/src

./src/backend/vendor/bin/php-cs-fixer fix ./src/backend/config
./src/backend/vendor/bin/php-cs-fixer fix ./src/backend/public
./src/backend/vendor/bin/php-cs-fixer fix ./src/backend/src

# Python
./src/python-backend/.venv/bin/black ./src/python-backend/main.py
./src/python-backend/.venv/bin/black ./src/python-backend/app

# Svelte
cd ./src/frontend/src/
npx prettier --write .
cd ../../../
