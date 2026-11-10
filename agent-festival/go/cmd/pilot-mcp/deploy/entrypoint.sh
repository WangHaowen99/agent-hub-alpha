#!/bin/sh
set -e
python /usr/local/bin/aha-configure.py
exec jiuwenswarm-start all
