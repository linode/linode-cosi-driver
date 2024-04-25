#!/usr/bin/env bash

set -a
set -e
set -x

pre-commit autoupdate
pre-commit install --hook-type pre-commit
pre-commit install --hook-type commit-msg
