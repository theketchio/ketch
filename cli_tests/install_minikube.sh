#!/bin/sh

set -e

export CHANGE_MINIKUBE_NONE_USER=true
export MINIKUBE_WANTUPDATENOTIFICATION=false
export MINIKUBE_WANTREPORTERRORPROMPT=false
export MINIKUBE_HOME=$HOME
export KUBECONFIG=$HOME/.kube/config

# Minikube
sudo apt-get update -yq && sudo apt-get -qq -y install conntrack
curl -Lo minikube https://storage.googleapis.com/minikube/releases/v1.20.0/minikube-linux-amd64
chmod +x minikube
sudo mv minikube /usr/local/bin/
mkdir -p $HOME/.kube $HOME/.minikube
touch "$KUBECONFIG"