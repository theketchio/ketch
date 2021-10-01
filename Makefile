
# Image URL to use all building/pushing image targets
IMG ?= controller:latest
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true,crdVersions=v1"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

GOOS=$(shell go env GOOS)
GOARCH=$(shell go env GOARCH)

KUBECTL_VERSION=v1.22.2
KUBECTL_INSTALL_DIR ?= "/usr/local/bin"

KUBEAPI_SERVER_VERSION=v1.22.2
KUBEAPI_SERVER_INSTALL_DIR ?= "/usr/local/bin"

ETCD_VERSION=v3.4.16
ETCD_INSTALL_DIR ?= "/usr/local/bin"

KUBEBUILDER_VERSION=3.1.0
KUBEBUILDER_INSTALL_DIR ?= "/usr/local/bin"
KUBEBUILDER_RELEASE=kubebuilder_${GOOS}_${GOARCH}

KUSTOMIZE_VERSION=4.3.0
KUSTOMIZE ?= $(shell which kustomize)
KUSTOMIZE_INSTALL_DIR ?= "/usr/local/bin"

.PHONY: all
all: manager ketch

# Run tests
.PHONY: test
test: generate fmt vet manifests
	go test ./... -coverprofile cover.out

# Build manager binary
.PHONY: manager
manager: generate fmt vet
	go build -o bin/manager cmd/manager/main.go

# Build ketch binary
.PHONY: ketch
ketch: generate fmt vet
	go build -o bin/ketch ./cmd/ketch/

# Run against the configured Kubernetes cluster in ~/.kube/config
.PHONY: run
run: generate fmt vet manifests
	go run ./cmd/manager/main.go

# Install CRDs into a cluster
.PHONY: manifests
install: manifests
	kustomize build config/crd | kubectl apply -f -

.PHONY: install-kubectl
install-kubectl:
	curl -LO "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/${GOOS}/${GOARCH}/kubectl"
	chmod 777 kubectl && sudo mv kubectl ${KUBECTL_INSTALL_DIR}

.PHONY: install-kube-apiserver
install-kube-apiserver:
	curl -LO https://storage.googleapis.com/kubernetes-release/release/${KUBECTL_VERSION}/bin/${GOOS}/${GOARCH}/kube-apiserver
	chmod 777 kube-apiserver && sudo mv kube-apiserver ${KUBEAPI_SERVER_INSTALL_DIR}

.PHONY: install-etcd
install-etcd:
	curl -L -O "https://github.com/etcd-io/etcd/releases/download/${ETCD_VERSION}/etcd-${ETCD_VERSION}-${GOOS}-${GOARCH}.tar.gz"
	tar -zxvf etcd-${ETCD_VERSION}-${GOOS}-${GOARCH}.tar.gz && mv etcd-${ETCD_VERSION}-${GOOS}-${GOARCH}/etcd etcd
	chmod 777 etcd && sudo mv etcd ${ETCD_INSTALL_DIR}
	rm etcd-${ETCD_VERSION}-${GOOS}-${GOARCH}.tar.gz
	rm -rf etcd-${ETCD_VERSION}-${GOOS}-${GOARCH}

.PHONY: install-kubebuilder
install-kubebuilder: install-kubectl install-kube-apiserver install-etcd
	curl -L -O "https://github.com/kubernetes-sigs/kubebuilder/releases/download/v${KUBEBUILDER_VERSION}/${KUBEBUILDER_RELEASE}"
	mv ${KUBEBUILDER_RELEASE} kubebuilder && chmod 777 kubebuilder && sudo mv kubebuilder ${KUBEBUILDER_INSTALL_DIR}

.PHONY: install-kustomize
install-kustomize:
	curl -s "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh" | bash -s ${KUSTOMIZE_VERSION}
	sudo mv kustomize ${KUSTOMIZE_INSTALL_DIR}/

# Uninstall CRDs from a cluster
.PHONY: uninstall
uninstall: manifests
	kustomize build config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
.PHONY: deploy
deploy: manifests
	cd config/manager && kustomize edit set image controller=${IMG}
	kustomize build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
.PHONY: manifests
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
.PHONY: fmt
fmt:
	go fmt ./...

# Run go vet against code
.PHONY: vet
vet:
	go vet ./...

# Generate code
.PHONY: generate
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="internal/hack/boilerplate.go.txt" paths="./internal/api/v1beta1/"
	go run internal/templates/generator/main.go

# Build the docker image
.PHONY: docker-build
docker-build: test
	docker build . -t ${IMG}

# Push the docker image
.PHONY: docker-push
docker-push:
	docker push ${IMG}

.PHONY: create-controller-yaml
create-controller-yaml:
	cd config/manager && ${KUSTOMIZE} edit set image controller=${IMG} && cd ../../
	${KUSTOMIZE} build config/default > ketch-controller.yaml

# find or download controller-gen
# download controller-gen if necessary
.PHONY: controller-gen
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.4.1 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif
