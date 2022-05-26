## Contributing to Ketch
We welcome contributions to Ketch in the form of Pull Requests or submitting Issues. Looking for your first contribution? There
are excellent [starter issues](https://github.com/theketchio/ketch/labels/good%20first%20issue) to work on. If you encounter a problem with Ketch or want
to suggest an improvement please submit an Issue. If you find a bug please tell us about it by submitting an Issue or
Pull Request.  Please make sure you are testing against the latest version of Ketch when you are submitting a bug. Provide
[as much detail](https://github.com/theketchio/ketch/blob/main/.github/pull_request_template.md) as you can.  Examples of detail would be the operating system you are using, the version of Kubernetes,
stack traces, the command(s) that caused the bug and so on.  

## Developer Guide
### Prerequisites
[Docker](https://docs.docker.com/get-docker/)

[Go version 1.14 or better](https://golang.org/dl/)

[Kubectl and Kubernetes version 1.17.1 or better](https://kubernetes.io/docs/tasks/tools/install-kubectl/)

[Minikube](https://minikube.sigs.k8s.io/docs/start/) (optional)

[Kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) Install with `make install-kubebuilder`.

[Kustomize](https://github.com/kubernetes-sigs/kustomize) Install with `make install-kustomize`.

### Developer Setup
In this example we build and install Ketch. Clone the project. From the project directory run `make install-kubebuilder` and `make install-kustomize`.  After doing that you should be able to run the unit tests `make test` successfully.

> If you experience issues with running `make test`, you may not have all the needed dependencies
>
> The following assets are required:
> 1. `/usr/local/kubebuilder/bin/etcd`:
>    1. Options:
>       1. Download via https://github.com/etcd-io/etcd/releases/tag/v3.5.4
>       2. Build from source: `git clone https://github.com/etcd-io/etcd.git`, `make build`, `./bin/etcd version`, `sudo cp ./bin/etcd /usr/local/kubebuilder/bin`
> 2. `/usr/local/kubebuilder/bin/kube-apiserver`
>    1. Options:
>       1. Download via https://www.downloadkubernetes.com/ (v1.24.1)
>       2. Build from source: [Instructions](https://github.com/kubernetes/community/blob/master/contributors/devel/development.md
                ) `git clone https://github.com/kubernetes/kubernetes.git`, `make WHAT=cmd/kube-apiserver`, `sudo mv ./_output/bin/kube-apiserver /usr/local/kubebuilder/bin`
> 3. `kube-apiserver` is contained in $PATH

### Developer Install with Minikube
Create Ketch controller image. The example assumes you have an minikube instance running (`minikube start`).

```bash
export IMG=my-repo/imagename:v0.1
make docker-build
make docker-push
```
Install Ketch controller.

```bash
make deploy
```
Build the Ketch CLI.

```bash
make ketch
export PATH=$(pwd)/bin:$PATH
```

Set up a route so the Minikube network is accessible.

```bash
sudo route add -host $(kubectl get svc traefik -o jsonpath='{.spec.clusterIP}') gw $(minikube ip)
```

Use the cluster IP address when you create frameworks.

```bash
ketch framework add myframework --ingress-service-endpoint $(kubectl get svc traefik -o jsonpath='{.spec.clusterIP}')
```

### Running the Ketch Controller locally with Minikube
Start a Minikube cluster:

`minikube start -p ketch-cluster --driver=hyperkit`

Install CRDs:

`make install`

*Note: If you revise any CRDs or experience trouble, you can install them manually from the `config/crd` directory, e.g. `kubectl apply -f config/crd/bases/theketch.io_frameworks.yaml`*

Create Ketch's namespace:

`kubectl create ns ketch-system`

Run the controller locally. This permits you to see logs during development:

`go run ./cmd/manager/main.go --enable-leader-election=false --disable-webhooks`

Run a CLI command (in a seperate terminal window):

`go run ./cmd/ketch/. framework add test`

During development, you may need to utilize some `make` utils to update generated code/templates:

- `make generate` to update modified helm templates.
- `make uninstall` to uninstall CRDs prior to re-running `make install`.
- `make manifests` to generate CRDs & RBAC from from code annotations. Ketch uses [Kubebuilder](https://book.kubebuilder.io/reference/controller-gen.html).

## Submission Guidelines
Ketch follows a lightweight Pull Request process. When submitting a PR, answering a few [basic questions](https://github.com/theketchio/ketch/blob/main/.github/pull_request_template.md) around type of change and steps to test and you are well on your way to a PR approval.   

## Your First Ketch Pull Request
Please Fork the project and create a branch to make your changes to. Directly commit your changes to your branch and then when 
ready to merge upstream, feel free to and create a PR. There are quality steps that need to be achieved. 

`git checkout -b add-ketch-feature-of-squash-bug`

### Running Tests
As a good practice, locally running the integration tests is a good idea. Because during the CI Step e.g GitHub Actions, these will be
run again as a quality gate. 

#### Unit Tests
Unit test coverage is in place. Running "make test" will run all of the Unit Tests e.g "Go Test". 
`make test`

Test coverage is represented as files suffixed as "test". A good idea is to include Unit Tests
with your changes. 

[Example Unit Test](https://github.com/theketchio/ketch/blob/main/internal/controllers/app_controller_test.go)

#### Ketch CLI
The CLI has integration tests if you are making CLI level changes. These tests will run the
CLI against a live Ketch Installation. 

`./cli_tests/app.sh`
[Ketch App CLI Tests](https://github.com/theketchio/ketch/blob/main/cli_tests/app.sh#L3-L8)

`./cli_tests/job.sh`
[Ketch Jobs CLI Tests](https://github.com/theketchio/ketch/blob/main/cli_tests/job.sh#L3-L8)

### Submitting PR
When you commit via GitHub, there will be GitHub Actions that is ran on your behalf. 
[GitHub Actions Config](https://github.com/theketchio/ketch/blob/main/.github/workflows/deploy.yaml#L50-L89)

The GitHub Actions will create a temporary K3d cluster on the GitHub Actions Agent to run the tests. 

Assuming everything passes in your branch, you are now ready to create a Pull Request and get ready for the merge. 

#### PR Acceptance
Currently, one Core Contributor will have to accept the Pull Request. Thanks for your PR!
