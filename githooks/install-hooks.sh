#!/bin/bash -aex

pre-commit install --hook-type pre-commit
pre-commit install --hook-type commit-msg
