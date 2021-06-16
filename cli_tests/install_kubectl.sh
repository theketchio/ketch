#!/bin/sh

set -e

curl -Lo kubectl https://storage.googleapis.com/kubernetes-release/release/v1.20.1/bin/linux/amd64/kubectl
chmod +x kubectl
sudo mv kubectl /usr/local/bin/
