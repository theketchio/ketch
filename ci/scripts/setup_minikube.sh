TRAVIS_COMMIT=$1
DOCKER_USERNAME=$2
DOCKER_PASSWORD=$3

echo "TESTING... $TRAVIS_COMMIT $DOCKER_USERNAME"
## Minikube
#- curl -Lo kubectl https://storage.googleapis.com/kubernetes-release/release/v1.20.1/bin/linux/amd64/kubectl && chmod +x kubectl && sudo mv kubectl /usr/local/bin/
#- curl -Lo minikube https://storage.googleapis.com/minikube/releases/v1.16.0/minikube-linux-amd64 && chmod +x minikube && sudo mv minikube /usr/local/bin/
#- mkdir -p $HOME/.kube $HOME/.minikube
#- touch "$KUBECONFIG"
#- sudo minikube start --profile=minikube --vm-driver=none --kubernetes-version=v1.20.1
#- minikube update-context --profile=minikube
#- "sudo chown -R travis: /home/travis/.minikube/"
#- eval "$(minikube docker-env --profile=minikube)" && export DOCKER_CLI='docker'
## TODO - rm and rely on push from docker repo, make type == PR
#- echo "$DOCKER_PASSWORD" | docker login --username "$DOCKER_USERNAME" --password-stdin
#- docker build -t shipasoftware/ketch:$TRAVIS_COMMIT .
#- docker push shipasoftware/ketch:$TRAVIS_COMMIT
## Kubebuiler & Kustomize
#- make install-kubebuilder KUBEBUILDER_INSTALL_DIR=/tmp/kubebuilder
#- make install-kustomize KUSTOMIZE_INSTALL_DIR=/tmp
#- export PATH=$PATH:/tmp
## Helm
#- curl https://baltocdn.com/helm/signing.asc | sudo apt-key add -
#- sudo apt-get install apt-transport-https --yes
#- echo "deb https://baltocdn.com/helm/stable/debian/ all main" | sudo tee /etc/apt/sources.list.d/helm-stable-debian.list
#- sudo apt-get update
#- sudo apt-get install helm