#!/bin/sh

export 'BACKEND_URL=http://localhost:8090/api/'

rm -fr build/
make
./build/abcmovies.out
