# Ketch : Deployment Script for CI/CD

This script can be used to deploy of your apps right from your CI/CD pipelines using [Ketch](theketch.io). 

### Usage

```
Usage: ./deploy.sh [-t --ketch-tag] [-o --pool] [-ig --ingress] [--endpoint] [-a --app] [-i --image] [-e --env] [-ig --ingress] [--registry-secret] [--ketch-yaml] [--procfile]
```

Instructions on how to use it are given below.

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


