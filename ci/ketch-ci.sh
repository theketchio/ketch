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

# set defaults if not set by user
if [ -z "$KETCH_TAG" ]; then 
    KETCH_TAG=$(curl -s https://api.github.com/repos/shipa-corp/ketch/releases/latest | grep -Eo '"tag_name":.*[^\\]",' | head -n 1 | sed 's/[," ]//g' | cut -d ':' -f 2)
fi

# Install ketch binary at /usr/local/bin default location
 curl -s https://raw.githubusercontent.com/shipa-corp/ketch/main/install.sh | TAG="${KETCH_TAG}" bash

# Install Ketch controller
kubectl apply -f https://github.com/shipa-corp/ketch/releases/download/"${KETCH_TAG}"/ketch-controller.yaml

if [ "$RESOURCE_CREATION" = true ] ; then
    # validate addtional required params
    if [ -z "$POOL" ]; then usage "Pool name required"; fi;
    if [ -z "$INGRESS_ENDPOINT" ]; then usage "Ingress endpoint required"; fi;

    # Add a pool with ingress Traefik (default), replace ingress endpoint address by your ingress IP address
    echo "creating pool for deployment ..."
    ketch pool add "${POOL}"  --ingress-service-endpoint "${INGRESS_ENDPOINT}" --ingress-type "${INGRESS_TYPE}"

    # Create app
    echo "creating app for deployment ..."
    CREATE_CMD="ketch app create ${APP_NAME} --pool ${POOL}" 

    if [ ! -z "$APP_ENV" ]; then
        CREATE_CMD+=" --env ${APP_ENV}"
    fi

    if [ ! -z "$REG_SECRET" ]; then
        CREATE_CMD+=" --registry-secret ${REG_SECRET}"
    fi

    echo "CMD: ${CREATE_CMD}"
    eval "${CREATE_CMD}"
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