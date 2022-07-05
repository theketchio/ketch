#!/usr/bin/env bats

# To run locally:
# export KETCH_EXECUTABLE_PATH=<location of ketch binary>
# assure you have a kubernetes cluster running w/ traefik, cert manager, etc. (see ketch getting started docs)
# assure the ketch cli is compiled (make ketch)
# assure you have bats installed locally (via apt, brew, etc.)
# ./cli_tests/app.sh

setup() {
  if [[ -z "${KETCH_EXECUTABLE_PATH}" ]]; then
    KETCH=$(pwd)/bin/ketch
  else
    KETCH="${KETCH_EXECUTABLE_PATH}"
  fi

  JOB_NAMESPACE="jobnamespace"
  JOB_NAME="sample-job"
}

teardown() {
  rm -f job.yaml
}

@test "job help" {
  result="$($KETCH job --help)"
  echo "RECEIVED:" $result
  [[ $result =~ "deploy" ]]
  [[ $result =~ "list" ]]
  [[ $result =~ "export" ]]
  [[ $result =~ "remove" ]]
}

@test "job deploy with yaml file" {
  nsresult=$(kubectl create ns "$JOB_NAMESPACE")
  echo "RECEIVED:" $nsresult
  [[ $nsresult =~ "namespace/$JOB_NAMESPACE created" ]]

  cat << EOF > job.yaml
name: "$JOB_NAME"
version: v1
type: Job
namespace: "$JOB_NAMESPACE"
description: "cli test job"
containers:
  - name: pi
    image: perl
    command:
      - "perl"
      - "-Mbignum=bpi"
      - "-wle"
      - "print bpi(2000)"
parallelism: 2
EOF
  result=$($KETCH job deploy job.yaml)
  [[ $result =~ "Successfully added!" ]]

  dataRegex="$JOB_NAME[ \t]+v1[ \t]+$JOB_NAMESPACE[ \t]+cli test job"
  result=$($KETCH job list $JOB_NAME)
  echo "RECEIVED:" $result
  [[ $result =~ $dataRegex ]]
}

@test "job list" {
  result=$($KETCH job list)
  headerRegex="NAME[ \t]+VERSION[ \t]+NAMESPACE[ \t]+DESCRIPTION"
  dataRegex="$JOB_NAME[ \t]+v1[ \t]+$JOB_NAMESPACE[ \t]+cli test job"
  echo "RECEIVED:" $result
  [[ $result =~ $headerRegex ]]
  [[ $result =~ $dataRegex ]]
}

@test "job export" {
  run $KETCH job export "$JOB_NAME" -f job.yaml
  result=$(cat job.yaml)
  echo "RECEIVED:" $result
  [[ $result =~ "name: $JOB_NAME" ]]
  [[ $result =~ "type: Job" ]]
  [[ $result =~ "namespace: $JOB_NAMESPACE" ]]
}

@test "job remove" {
  result=$($KETCH job remove "$JOB_NAME")
  echo "RECEIVED:" $result
  [[ $result =~ "Successfully removed!" ]]

  # clean up namespace
  fwresult=$(kubectl delete ns "$JOB_NAMESPACE")
  echo "RECEIVED:" $fwresult
  [[ $fwresult =~ "namespace \"$JOB_NAMESPACE\" deleted" ]]
}
