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
INGRESS_ENDPOINT=""
APP_NAME=""
DOCKER_IMAGE=""

while [[ $# -gt 0 ]]
do
key="$1"

# set variables with custom user inputs
case $key in
    -t|--ketch_tag)
    KETCH_TAG="$2"
    shift # past argument
    shift # past value
    ;;
    -p|--pool)
    POOL="$2"
    shift # past argument
    shift # past value
    ;;
    -ig|--ingress)
    INGRESS_TYPE="$2"
    shift # past argument
    shift # past value
    ;;
    -e|--endpoint)
    INGRESS_ENDPOINT="$2"
    shift # past argument
    shift # past value
    ;;
    -a|--app)
    APP_NAME="$2"
    shift # past argument
    shift # past value
    ;;
    -i|--image)
    DOCKER_IMAGE="$2"
    shift # past argument
    shift # past value
    ;;
    *)    # unknown option
    echo "unknown option $1"
    exit 1
    ;;
esac
done

# Install latest ketch binary at /usr/local/bin default location
curl -s https://raw.githubusercontent.com/shipa-corp/ketch/main/install.sh | TAG="${KETCH_TAG}" bash

# Install Ketch controller
kubectl apply -f https://github.com/shipa-corp/ketch/releases/download/v0.1.0/ketch-controller.yaml

# Add a pool with ingress Traefik (default), replace ingress endpoint address by your ingress IP address
ketch pool add "${POOL}"  --ingress-service-endpoint "${INGRESS_ENDPOINT}" --ingress-type "${INGRESS_TYPE}"

# Create app
ketch app create "${APP_NAME}" --pool "${POOL}"   

# Deploy app
ketch app deploy "${APP_NAME}" -i "${DOCKER_REGISTRY}"