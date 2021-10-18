#!/bin/sh

# exit when any command fails
set -e
set -o pipefail
set -x
# keep track of the last executed command
trap 'last_command=$current_command; current_command=$BASH_COMMAND' DEBUG
# echo an error message before exiting
trap 'echo "\"${last_command}\" command filed with exit code $?."' EXIT

# start cluster
sudo minikube start --profile=minikube --vm-driver=none --kubernetes-version=v1.20.1
sudo chown -R travis /home/travis/.minikube/

# kubebuilder
make install-kubebuilder KUBEBUILDER_INSTALL_DIR=/tmp/kubebuilder

# kustomize
make install-kustomize KUSTOMIZE_INSTALL_DIR=/tmp
export PATH=$PATH:/tmp

# ketch
kubectl cluster-info
make manifests install ketch manager
export PATH=$PATH:$(pwd)/bin
ketch -v

# helm
curl -fsSL -o get_helm.sh https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 && chmod 700 get_helm.sh && ./get_helm.sh

# cert-manager
kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.3.1/cert-manager.yaml

# traefik
helm repo add traefik https://helm.traefik.io/traefik
helm repo update
helm install traefik traefik/traefik

# istio
ISTIO_VERSION=1.11.0 && curl -L -k https://istio.io/downloadIstio |ISTIO_VERSION=1.11.0 sh - && cd istio-$ISTIO_VERSION && ./bin/istioctl install --set profile=demo -y

# nginx
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo update
helm install ingress-nginx ingress-nginx/ingress-nginx

# wait for containers
kubectl wait --for=condition=Ready=true pod -n cert-manager --all
kubectl get pods -A

