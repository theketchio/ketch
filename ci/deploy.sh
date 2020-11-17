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

# set colors for printing texts
CLEAR='\033[0m'
RED='\033[0;31m'

# prints usage
function usage() {
  if [ -n "$1" ]; then
    echo -e "${RED}👉 $1${CLEAR}\n";
  fi

  echo "Usage: $0 [-t --ketch-tag] [-p --pool] [-ig --ingress] [-e --endpoint] [-a --app] [-i --image]"
  echo "  -t, --ketch-tag   Ketch version. Default is latest."
  echo "  -p, --pool           Pool Name."
  echo "  -a, --app            App Name."
  echo "  -ig, --ingress     Ingress type. Default is Traefik."
  echo "  -e, --endpoint    Ingress IP address."
  echo "  -i, --image          Docker image for the application."
  exit 1
}

# parse params and set variables with custom user inputs
while [[ "$#" > 0 ]]; do case $1 in
    -t|--ketch-tag) KETCH_TAG="$2"; shift;shift;;
    -p|--pool) POOL="$2"; shift;shift;;
    -ig|--ingress) INGRESS_TYPE="$2"; shift;shift;;
    -e|--endpoint) INGRESS_ENDPOINT="$2"; shift;shift;;
    -a|--app) APP_NAME="$2"; shift;shift;;
    -i|--image) DOCKER_IMAGE="$2"; shift;shift;;
    *) usage "Unknown parameter passed: $1"; shift; shift;;
esac; done

# validate params
if [ -z "$POOL" ]; then usage "Pool name required"; fi;
if [ -z "$INGRESS_ENDPOINT" ]; then usage "Ingress endpoint required"; fi;
if [ -z "$APP_NAME" ]; then usage "App Name required"; fi;
if [ -z "$DOCKER_IMAGE" ]; then usage "Image for the app is required"; fi;


# Install ketch binary at /usr/local/bin default location
curl -s https://raw.githubusercontent.com/shipa-corp/ketch/main/install.sh | TAG="${KETCH_TAG}" bash

# Install Ketch controller
kubectl apply -f https://github.com/shipa-corp/ketch/releases/download/v0.1.0/ketch-controller.yaml

# Add a pool with ingress Traefik (default), replace ingress endpoint address by your ingress IP address
ketch pool add "${POOL}"  --ingress-service-endpoint "${INGRESS_ENDPOINT}" --ingress-type "${INGRESS_TYPE}"

# Create app
ketch app create "${APP_NAME}" --pool "${POOL}"   

# Deploy app
ketch app deploy "${APP_NAME}" -i "${DOCKER_REGISTRY}"