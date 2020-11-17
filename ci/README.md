# Ketch : Deployment Script for CI/CD

This script can be used to deploy your apps right from your CI/CD pipelines using [Ketch](theketch.io). 

### Usage

```
Usage: ./deploy.sh [-t --ketch-tag] [-o --pool] [-ig --ingress] [--endpoint] [-a --app] [-i --image] [-e --env] [-ig --ingress] [--registry-secret] [--ketch-yaml] [--procfile]
```

| Flags | Descriptions |
| ------ | ------ |
|  -t, --ketch-tag  | Ketch version. Default is latest. |
|  -o, --pool        |     Pool where your application should be deployed.|
|  -a, --app         |     Application Name.|
| -e, --env          |     Application environment variables.|
| -ig, --ingress   |     Ingress type. Default is Traefik. |
| --endpoint       |      Ingress IP address.|
|  -i, --image      |     The image that should be used with the application.|
|  --registry-secret  |    A name of a Secret with docker credentials. This secret must be created in the same namespace of the pool.|
|  --ketch-yaml        |   The path to the ketch.yaml file.|
|  --procfile          |   The path to Procfile. If not set, ketch will use the entrypoint and cmd from the image.
|  --skip-resource-creation       | If set, ketch will NOT create app and pool for the deployment. Useful when resources already exist. |


Examples on how to use it with various CI providers are given below.

#### Travis CI
`.travis.yaml`

```
......
............
jobs:
  include:
    - stage: build
      script: ./build.sh     # build and push docker images
    - stage: deploy
      script: ./deploy.sh --ketch-tag v0.1.0 -a myapp -o mypool --endpoint 104.155.134.17 -i docker.io/shipasoftware/bulletinboard:1.0 --ingress traefik
```