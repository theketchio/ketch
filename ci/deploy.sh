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
KETCH_YAML=""
PROCFILE=""

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
  echo "  -o, --pool           Pool Name."
  echo "  -a, --app            App Name."
  echo "  -ig, --ingress     Ingress type. Default is Traefik."
  echo "  -e, --endpoint   Ingress IP address."
  echo "  -i, --image         Docker image for the application."
  echo "  --ketch-yaml     The path to the ketch.yaml file."
  echo "  --procfile	         The path to Procfile. If not set, ketch will use the entrypoint and cmd from the image."
  exit 1
}

# parse params and set variables with custom user inputs
while [[ "$#" > 0 ]]; do case $1 in
    -t|--ketch-tag) KETCH_TAG="$2"; shift;shift;;
    -o|--pool) POOL="$2"; shift;shift;;
    -ig|--ingress) INGRESS_TYPE="$2"; shift;shift;;
    -e|--endpoint) INGRESS_ENDPOINT="$2"; shift;shift;;
    -a|--app) APP_NAME="$2"; shift;shift;;
    -i|--image) DOCKER_IMAGE="$2"; shift;shift;;
    --ketch-yaml) KETCH_YAML="$2"; shift;shift;;
    --procfile) PROCFILE="$2"; shift;shift;;
    *) usage "Unknown parameter passed: $1"; shift; shift;;
esac; done


# validate params
if [ -z "$POOL" ]; then usage "Pool name required"; fi;
if [ -z "$INGRESS_ENDPOINT" ]; then usage "Ingress endpoint required"; fi;
if [ -z "$APP_NAME" ]; then usage "App Name required"; fi;
if [ -z "$DOCKER_IMAGE" ]; then usage "Image for the app is required"; fi;

# set defaults if not set by user
if [ -z "$KETCH_TAG" ]; then 
    KETCH_TAG=$(curl -s https://api.github.com/repos/shipa-corp/ketch/releases/latest | grep -Eo '"tag_name":.*[^\\]",' | head -n 1 | sed 's/[," ]//g' | cut -d ':' -f 2)
fi

# Install ketch binary at /usr/local/bin default location
curl -s https://raw.githubusercontent.com/shipa-corp/ketch/main/install.sh | TAG="${KETCH_TAG}" bash

# Install Ketch controller
kubectl apply -f https://github.com/shipa-corp/ketch/releases/download/"${KETCH_TAG}"/ketch-controller.yaml

# Add a pool with ingress Traefik (default), replace ingress endpoint address by your ingress IP address
ketch pool add "${POOL}"  --ingress-service-endpoint "${INGRESS_ENDPOINT}" --ingress-type "${INGRESS_TYPE}"

# Create app
ketch app create "${APP_NAME}" --pool "${POOL}"   

# Deploy app
ketch app deploy "${APP_NAME}" -i "${DOCKER_REGISTRY}" --ketch-yaml="${KETCH_YAML}" --procfile="${PROCFILE}"