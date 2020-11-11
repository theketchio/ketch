![Ketch](https://i.imgur.com/TVe46Dm.png)


[![Build Status](https://travis-ci.com/shipa-corp/ketch.svg?token=qcHta8a4Eyd9eGNDTuSN&branch=main)](https://travis-ci.com/shipa-corp/ketch) 
[![Slack](https://img.shields.io/badge/chat-on%20slack-6A5DAB)](https://shipa-io.slack.com/archives/C01E4FMEY9K)

Think applications not yamls :)

# What is Ketch

Ketch is a tool that makes it easy to deploy and manage applications on Kubernetes using a simple command line interface.
No YAML required! 

## Architecture Diagram 
![Architecture](./img/ketch-architecture.png)

## Getting Started

### Download and Install Ketch 
The latest Ketch release can be found [here](https://github.com/shipa-corp/ketch/releases). Use the following commands
to install Ketch, changing the version in the commands to match the version of Ketch you want to install. 
 
For Linux use the following commands to download and install the Ketch cli. 
```bash
curl -o ketch https://github.com/shipa-corp/ketch/releases/download/v0.0.1-beta-9/ketch_0.0.1-beta-9_linux_amd64
chmod +x ./ketch && mv ./ketch /usr/local/bin 
```

For Macs use the following commands to download and install the Ketch cli. 
```bash
curl -o ketch https://github.com/shipa-corp/ketch/releases/download/v0.0.1-beta-9/ketch_0.0.1-beta-9_darwin_amd64
chmod +x ./ketch && mv ./ketch /usr/local/bin 
```
Use [Helm](https://helm.sh/docs/intro/install/) to install Traefik. 

```bash 
helm repo add traefik https://helm.traefik.io/traefik
helm repo update
helm install traefik traefik/traefik
```
Install Cert Manager.
```bash
kubectl apply --validate=false -f https://github.com/jetstack/cert-manager/releases/download/v1.0.3/cert-manager.yaml
```
Install Ketch controller.
```bash
kubectl -f https://github.com/shipa-corp/ketch/releases/download/v0.1.0-beta-6/ketch-controller.yaml
```
Thats it!

## Using Ketch 
### Quick Start
Deploying apps is easy once you've installed Ketch.  First, create a pool. Then create app(s) adding them to the pool and finally 
deploy the app(s).  The following example illustrates these steps. 

```bash
# Add a pool with ingress Traefik (default)
ketch pool add mypool  --ingress-service-endpoint 35.247.8.23 --ingress-type traefik

# Create app
ketch app create bulletinboard --pool mypool       

# Deploy app
ketch app deploy bb1 -i docker.io/shipasoftware/bulletinboard:1.0 

# Check app status
ketch app list 

NAME             POOL    STATUS     UNITS    ADDRESSES                                DESCRIPTION
bulletinboard    mypool     Running    1        bulletinboard.35.247.8.23.shipa.cloud    
```
After you deploy your application, you can access it at the address associated with it using the `ketch app list`, in 
this example `bulletinboard.35.247.8.23.shipa.cloud`. 

### Usage 

```bash
For details see https://theketch.io

Usage:
  ketch [flags]
  ketch [command]

Available Commands:
  app         Manage applications
  cname       Manage cnames of an application
  env         Manage an app's environment variables
  help        Help about any command
  pool        Manage pools
  unit        Manage an app's units

Flags:
  -h, --help   help for ketch

Use "ketch [command] --help" for more information about a command.

```

#### Developer Guide [See Contributing](./CONTRIBUTING.md)