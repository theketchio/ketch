![Ketch](https://i.imgur.com/TVe46Dm.png)


[![Build Status](https://travis-ci.com/shipa-corp/ketch.svg?token=qcHta8a4Eyd9eGNDTuSN&branch=main)](https://travis-ci.com/shipa-corp/ketch) 
[![Slack](https://img.shields.io/badge/chat-on%20slack-6A5DAB)](https://shipa-io.slack.com/archives/C01E4FMEY9K)

Think applications not yamls :)

# What is Ketch
Ketch is an application delivery framework that facilitates the deployment and management of applications on Kubernetes using a simple command line interface. No YAML required!

## Architecture Diagram 
![Architecture](./img/ketch-architecture.png)

## Getting Started

### Download and Install Ketch 
The latest Ketch release can be found [here](https://github.com/shipa-corp/ketch/releases). Use the following commands
to install Ketch, 

Install latest at /usr/local/bin default location

```bash
curl -s https://raw.githubusercontent.com/shipa-corp/ketch/main/install.sh | bash
```

Alternatively, you can install specific tag at a target location, for example command below installs ketch version v0.2.0 in current directory:

```bash
curl -s https://raw.githubusercontent.com/shipa-corp/ketch/main/install.sh | INSTALL_DIR=. TAG=v0.2.0  bash
```



### Install Ingress Controller

At present, Ketch supports Istio and Traefik ingress controllers.

Here is how you can install Traefik:

Use [Helm](https://helm.sh/docs/intro/install/) to install Traefik. 

```bash 
helm repo add traefik https://helm.traefik.io/traefik
helm repo update
helm install traefik traefik/traefik
```

Or you can install Istio:

```bash
ISTIO_VERSION=1.9.0 && curl -L https://istio.io/downloadIstio |  sh - && cd istio-$ISTIO_VERSION && export PATH=$PWD/bin:$PATH
istioctl install --set profile=demo 
```

### Install Cert Manager.
```bash
kubectl apply --validate=false -f https://github.com/jetstack/cert-manager/releases/download/v1.0.3/cert-manager.yaml
```
### Install Ketch controller.
```bash
kubectl apply -f https://github.com/shipa-corp/ketch/releases/download/v0.2.0/ketch-controller.yaml
```

Thats it!

## Using Ketch 

Learn more about Ketch at [Ketch documentation](https://learn.theketch.io/docs)

### Quick Start
Deploying apps is easy once you've installed Ketch.  First, create a pool. Then create app(s) adding them to the pool and finally 
deploy the app(s).  The following example illustrates these steps. 

```bash
# Add a pool with ingress Traefik (default), replace ingress IP address by your ingress IP address
ketch pool add mypool  --ingress-service-endpoint 35.247.8.23 --ingress-type traefik

# Create app
ketch app create bulletinboard --pool mypool       

# Deploy app
ketch app deploy bulletinboard -i docker.io/shipasoftware/bulletinboard:1.0 

# Check app status
ketch app list 

NAME             POOL        STATE        ADDRESSES                                      PLATFORM    DESCRIPTION
bulletinboard    mypool      1 running    http://bulletinboard.35.247.8.23.shipa.cloud
```
After you deploy your application, you can access it at the address associated with it using the `ketch app list`, in 
this example `bulletinboard.35.247.8.23.shipa.cloud`. 

### Usage 
For details see https://theketch.io.

```bash
Usage:
  ketch [flags]
  ketch [command]

Available Commands:
  app         Manage applications
  cname       Manage cnames of an application
  env         Manage an app's environment variables
  help        Help about any command
  platform    Manage platforms
  pool        Manage pools
  unit        Manage an app's units

Flags:
  -h, --help      help for ketch
  -v, --version   version for ketch

Use "ketch [command] --help" for more information about a command.
```

#### Developer Guide [See Contributing](./CONTRIBUTING.md)
