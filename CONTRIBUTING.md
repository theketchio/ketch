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
## Submission Guidelines
Ketch follows a lightweight Pull Request process. When submitting a PR, answering a few [basic questions](https://github.com/theketchio/ketch/blob/main/.github/pull_request_template.md) around type of change and steps to test and you are well on your way to a PR approval.   
