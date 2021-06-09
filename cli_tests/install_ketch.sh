#!/bin/sh

# exit when any command fails
set -e
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
curl https://baltocdn.com/helm/signing.asc | sudo apt-key add -
sudo apt-get install apt-transport-https --yes
echo "deb https://baltocdn.com/helm/stable/debian/ all main" | sudo tee /etc/apt/sources.list.d/helm-stable-debian.list
sudo apt-get update
sudo apt-get install helm

# cert-manager
kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.3.1/cert-manager.yaml

# traefik
helm repo add traefik https://helm.traefik.io/traefik
helm repo update
helm install traefik traefik/traefik

# wait for containers
kubectl wait --for=condition=Ready=true pod -n cert-manager --all
kubectl get pods -A

# deploy
make deploy

