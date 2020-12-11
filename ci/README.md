# ketch-ci : Deployment Script for CI/CD

This script can be used to deploy your apps right from your CI/CD pipelines using [Ketch](theketch.io). 

### Prerequisites

The ingress controller (Traefik or Istio), cluster issuer, and cert-manager should be installed inside the cluster before using the script. If not already installed, then please follow the steps described [here](https://learn.theketch.io/docs/getting-started). Kubectl should be installed inside the runner.

### Usage

```
Usage: ./ketch-ci.sh [-t --ketch-tag] [-o --pool] [-ig --ingress] [--endpoint] [-a --app] [-i --image] [-e --env] [-ig --ingress] [--registry-secret] [--ketch-yaml] [--procfile] [--skip-resource-creation]
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
      script: ./ketch-ci.sh --ketch-tag v0.1.1 -a myapp -o mypool --endpoint 104.155.134.17 -i docker.io/shipasoftware/bulletinboard:1.0 --ingress traefik
```

#### Circle CI

`~/.circleci/config.yml`

```
......
............
deployment:
  production:
    branch: "master"
    commands:
      - ./ketch-ci.sh --ketch-tag v0.1.1 -a myapp -o mypool --endpoint 104.155.134.17 -i docker.io/shipasoftware/bulletinboard:1.0 --ingress traefik
```

#### Gitlab CI

`.gitlab-ci.yml`
```
......
............
docker:
    stage: build
    image: docker:stable
    services:
      - docker:dind
    when: on_success
    only:
      refs:
        - master

    script:
      - docker login -u gitlab-ci-token -p $CI_JOB_TOKEN $CI_REGISTRY
      - docker build -f Dockerfile -t=$CI_REGISTRY_IMAGE/myapp:latest .
      - echo "Pushing images to registry ..."
      - docker push $CI_REGISTRY_IMAGE/myapp:latest
      - docker system prune -f

production:
    stage: deploy   
    image: ubuntu:latest
    when: on_success
    only:
      - tags
    except:
    - branches

    script:
      - ./ketch-ci.sh --ketch-tag v0.1.1 -a myapp -o mypool --endpoint 104.155.134.17 -i docker.io/shipasoftware/bulletinboard:1.0 --ingress traefik
```