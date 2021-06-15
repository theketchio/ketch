#!/bin/sh

set -e

# BATS is a TAP-compliant CLI testing framework. See: https://github.com/bats-core/bats-core
sudo apt-get update -yq && sudo apt-get install bats -y
