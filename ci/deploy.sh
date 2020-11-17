#!/usr/bin/env bash

# Exit script if an uninitialized variable is used.
set -o nounset
# Exit script if a statement returns a non-true return value.
set -o errexit
# Use the error status of the first failure, rather than that of the last item in a pipeline.
set -o pipefail

# Set variables
KETCH_TAG=""
POOL=""
INGRESS_TYPE=""
INGRESS_IP=""
APP_NAME=""
DOCKER_REGISTRY=""

# Install latest ketch binary at /usr/local/bin default location
curl -s https://raw.githubusercontent.com/shipa-corp/ketch/main/install.sh | TAG="${KETCH_TAG}" bash

# Install Cert Manager
kubectl apply --validate=false -f https://github.com/jetstack/cert-manager/releases/download/v1.0.3/cert-manager.yaml

# Install Ketch controller
kubectl apply -f https://github.com/shipa-corp/ketch/releases/download/v0.1.0/ketch-controller.yaml

# Add a pool with ingress Traefik (default), replace ingress IP address by your ingress IP address
ketch pool add "${POOL}"  --ingress-service-endpoint "${INGRESS_IP}" --ingress-type "${INGRESS_TYPE}"

# Create app
ketch app create "${APP_NAME}" --pool "${POOL}"   

# Deploy app
ketch app deploy "${APP_NAME}" -i "${DOCKER_REGISTRY}"