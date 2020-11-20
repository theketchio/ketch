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
APP_ENV=""
DOCKER_IMAGE=""
REG_SECRET=""
KETCH_YAML=""
PROCFILE=""

# if true, ketch will try to create app and pool for the deployment
RESOURCE_CREATION=true

# set colors for printing texts
CLEAR='\033[0m'
RED='\033[0;31m'

# prints usage
function usage() {
  if [ -n "$1" ]; then
    echo -e "${RED}👉 $1${CLEAR}\n";
  fi

  echo -e "Usage: $0 [-t --ketch-tag] [-o --pool] [-ig --ingress] [--endpoint] [-a --app] [-i --image] [-e --env] [-ig --ingress] [--registry-secret] [--ketch-yaml] [--procfile] [--skip-resource-creation]\n"
  echo "  -t, --ketch-tag                 Ketch version. Default is latest."
  echo "  -o, --pool                      Pool where your application should be deployed."
  echo "  -a, --app                       App Name."
  echo "  -e, --env                       Application environment variables."
  echo "  -ig, --ingress                  Ingress type. Default is Traefik."
  echo "  --endpoint                      Ingress IP address."
  echo "  -i, --image                     The image that should be used with the application."
  echo "  --registry-secret               A name of a Secret with docker credentials. This secret must be created in the same namespace of the pool."
  echo "  --ketch-yaml                    The path to the ketch.yaml file."
  echo "  --procfile	                  The path to Procfile. If not set, ketch will use the entrypoint and cmd from the image."
  echo "  --skip-resource-creation        If set, ketch will NOT create app and pool for the deployment. Useful when resources already exist."
  exit 1
}

# parse params and set variables with custom user inputs
while [[ "$#" > 0 ]]; do case $1 in
    -t|--ketch-tag) KETCH_TAG="$2"; shift;shift;;
    -o|--pool) POOL="$2"; shift;shift;;
    -ig|--ingress) INGRESS_TYPE="$2"; shift;shift;;
    --endpoint) INGRESS_ENDPOINT="$2"; shift;shift;;
    -a|--app) APP_NAME="$2"; shift;shift;;
    -e|--env) APP_ENV="$2"; shift;shift;;
    -i|--image) DOCKER_IMAGE="$2"; shift;shift;;
    --registry-secret) REG_SECRET="$2"; shift;shift;;
    --ketch-yaml) KETCH_YAML="$2"; shift;shift;;
    --procfile) PROCFILE="$2"; shift;shift;;
    --skip-resource-creation) RESOURCE_CREATION=false; shift;;
    *) usage "Unknown parameter passed: $1"; shift; shift;;
esac; done


# validate params
if [ -z "$APP_NAME" ]; then usage "App Name required"; fi;
if [ -z "$DOCKER_IMAGE" ]; then usage "Image for the app is required"; fi;

# set default ketch tag if not set by user
if [ -z "$KETCH_TAG" ]; then 
    KETCH_TAG=$(curl -s https://api.github.com/repos/shipa-corp/ketch/releases/latest | grep -Eo '"tag_name":.*[^\\]",' | head -n 1 | sed 's/[," ]//g' | cut -d ':' -f 2)
fi

# set default ingress type if not set by user
if [ -z  "$INGRESS_TYPE"  ]; then 
    INGRESS_TYPE="traefik"
fi

# Ensure that required resource has atleast N number of pods in running state
# usage: ensure_resource <name> <pod count> 
function ensure_resource() {
  local retries=5

while [[ $(kubectl get pods --field-selector=status.phase=Running --all-namespaces | grep "$1" | wc -l | xargs ) -lt "$2" ]]; do
    echo -e "Waiting for $1 to be ready ...."
    sleep 5
    ((retries--))
    if ((retries == 0 )); then
      echo -e "${RED}Failed to ensure $1 in the cluster!${CLEAR}"
      exit 1
    fi
  done

  echo "$1 looks good! 👍"
}

echo "ensuring all the requirements in the cluster..."
echo "checking for Cert Manager ..."
ensure_resource cert-manager 3

if [ "$INGRESS_TYPE" = "istio" ]; then
  echo "checking for istio-egressgateway ..."
  ensure_resource istio-egressgateway 1

  echo "checking for istio-ingressgateway ...."
  ensure_resource istio-ingressgateway 1

  echo "checking for istiod ...."
  ensure_resource istiod 1
fi


if [ "$INGRESS_TYPE" = "traefik" ]; then
    echo "checking for Traefik..."
    ensure_resource traefik 1
fi

# Install ketch binary at /usr/local/bin default location
curl -s https://raw.githubusercontent.com/shipa-corp/ketch/main/install.sh | TAG="${KETCH_TAG}" bash

# Install Ketch controller if not already installed or not in running state
if [[ -z "$(kubectl get ns | grep ketch-system)" ]] || [[ $(kubectl get pods --field-selector=status.phase=Running -n ketch-system | grep ketch | wc -l | xargs) -eq 0 ]]; then
   echo "ketch controller not found or not in running state!"
   echo "installing ketch controller..."
   kubectl apply -f https://github.com/shipa-corp/ketch/releases/download/"${KETCH_TAG}"/ketch-controller.yaml
fi

ensure_resource 'ketch-controller-manager' 1

if [ "$RESOURCE_CREATION" = true ] ; then
    # validate addtional required params
    if [ -z "$POOL" ]; then usage "Pool name required"; fi;
    if [ -z "$INGRESS_ENDPOINT" ]; then usage "Ingress endpoint required"; fi;

    # Add a pool with ingress Traefik (default), replace ingress endpoint address by your ingress IP address
    echo "creating pool for deployment ..."
    POOL_CMD="ketch pool add ${POOL} --ingress-service-endpoint ${INGRESS_ENDPOINT}" 

    if [ ! -z "$INGRESS_TYPE" ]; then
        POOL_CMD+=" --ingress-type ${INGRESS_TYPE}"
    fi

    echo "CMD: ${POOL_CMD}"
    eval "${POOL_CMD}"

    # Create app
    echo "creating app for deployment ..."
    APP_CMD="ketch app create ${APP_NAME} --pool ${POOL}" 

    if [ ! -z "$APP_ENV" ]; then
        APP_CMD+=" --env ${APP_ENV}"
    fi

    if [ ! -z "$REG_SECRET" ]; then
        APP_CMD+=" --registry-secret ${REG_SECRET}"
    fi

    echo "CMD: ${APP_CMD}"
    eval "${APP_CMD}"
    # wait for app creation
    sleep 2
fi

# Deploy app
echo "deploying app ..."
DEP_CMD="ketch app deploy ${APP_NAME} -i ${DOCKER_IMAGE}" 

if [ ! -z "$KETCH_YAML" ]; then
    DEP_CMD+=" --ketch-yaml ${KETCH_YAML}"
fi

if [ ! -z "$PROCFILE" ]; then
    DEP_CMD+=" --procfile ${PROCFILE}"
fi

echo "CMD: ${DEP_CMD}"
eval "${DEP_CMD}"