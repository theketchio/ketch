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
  INGRESS_TRAEFIK=$(kubectl get svc traefik -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
  INGRESS_NGINX=$(kubectl get svc ingress-nginx-controller -o jsonpath='{.status.loadBalancer.ingress[0].ip}' -n ingress-nginx)
  INGRESS_ISTIO=$(kubectl get svc istio-ingressgateway -n istio-system -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
  NAMESPACE="appnamespace"
  APP_IMAGE="gcr.io/shipa-ci/sample-go-app:latest"
  APP_NAME="sample-app"
  CNAME="my-cname.com"
  TEST_ENVVAR_KEY="FOO"
  TEST_ENVVAR_VALUE="BAR"
}

teardown() {
  rm -f app.yaml
}

@test "help" {
  result="$($KETCH help)"
  echo "RECEIVED:" $result
  [[ $result =~ "For details see https://theketch.io" ]]
  [[ $result =~ "Available Commands" ]]
  [[ $result =~ "Flags" ]]
}

@test "app deploy" {
  run $KETCH app deploy "$APP_NAME" --namespace "$NAMESPACE" -i "$APP_IMAGE"
  [[ $status -eq 0 ]]
}

@test "app deploy istio" {
  run $KETCH app deploy "$APP_NAME-istio" --namespace "$NAMESPACE-istio" -i "$APP_IMAGE"
  [[ $status -eq 0 ]]
}

@test "app deploy nginx" {
  run $KETCH app deploy "$APP_NAME-nginx" --namespace "$NAMESPACE-nginx" -i "$APP_IMAGE"
  [[ $status -eq 0 ]]
}

@test "app deploy with yaml file" {
  cat << EOF > app.yaml
name: "$APP_NAME-2"
version: v1
type: Application
image: "$APP_IMAGE"
namespace: "$NAMESPACE"
description: cli test app
EOF
  run $KETCH app deploy app.yaml
  [[ $status -eq 0 ]]

  # retry for "running" status
  count=0
  until [[ $count -ge 20 ]]
  do
    result=$($KETCH app info $APP_NAME-2)
    if [[ $result =~ "running" ]]
      then break
    fi
    count+=1
    sleep 7
  done

  dataRegex="1[ \t]+$APP_IMAGE[ \t]+web[ \t]+100%[ \t]+"
  result=$($KETCH app info $APP_NAME-2)
  echo "RECEIVED:" $result
  [[ $result =~ $dataRegex ]]
  [[ $result =~ "Application: $APP_NAME-2" ]]
  [[ $result =~ "Namespace: $NAMESPACE" ]]
  [[ $result =~ "Version: v1" ]]
  [[ $result =~ "Description: cli test app" ]]
}

@test "app unit set" {
 run $KETCH app deploy "$APP_NAME" --namespace "$NAMESPACE" -i "$APP_IMAGE" --units 3
 [[ $status -eq 0 ]]
  result=$(kubectl describe apps $APP_NAME)
  echo "RECEIVED:" $result
 [[ $result =~ "Units:  3" ]] # note two spaces
}

@test "app list" {
  result=$($KETCH app list)
  headerRegex="NAME[ \t]+NAMESPACE[ \t]+STATE[ \t]+ADDRESSES[ \t]+BUILDER[ \t]+DESCRIPTION"
  dataRegex="$APP_NAME[ \t]+$NAMESPACE[ \t]+(created|1 running)"
  echo "RECEIVED:" $result
  [[ $result =~ $headerRegex ]]
  [[ $result =~ $dataRegex ]]
}

@test "app info" {
  result=$($KETCH app info "$APP_NAME")
  headerRegex="DEPLOYMENT VERSION[ \t]+IMAGE[ \t]+PROCESS NAME[ \t]+WEIGHT[ \t]+STATE[ \t]+CMD"
  dataRegex="1[ \t]+$APP_IMAGE[ \t]+web[ \t]+100%[ \t]+(created|1 running)[ \t]"
  echo "RECEIVED:" $result
  [[ $result =~ $headerRegex ]]
  [[ $result =~ $dataRegex ]]
}

@test "app export" {
  run $KETCH app export "$APP_NAME" -f app.yaml
  result=$(cat app.yaml)
  echo "RECEIVED:" $result
  [[ $result =~ "name: $APP_NAME" ]]
  [[ $result =~ "type: Application" ]]
  [[ $result =~ "namespace: $NAMESPACE" ]]
  rm -f app.yaml
}

@test "app stop" {
  result=$($KETCH app stop "$APP_NAME")
  echo "RECEIVED:" $result
  [[ $result =~ "Successfully stopped!" ]]
}

@test "app start" {
  result=$($KETCH app start "$APP_NAME")
  echo "RECEIVED:" $result
  [[ $result =~ "Successfully started!" ]]
}

@test "app log" {
  run $KETCH app log "$APP_NAME"
  [[ $status -eq 0 ]]
}

@test "builder list" {
  result=$($KETCH builder list)
  headerRegex="VENDOR[ \t]+IMAGE[ \t]+DESCRIPTION"
  dataRegex="Google[ \t]+gcr.io/buildpacks/builder:v1[ \t]+GCP Builder for all runtimes"
  echo "RECEIVED:" $result
  [[ $result =~ $headerRegex ]]
  [[ $result =~ $dataRegex ]]
}

@test "cname add" {
  run $KETCH cname add "$CNAME" --app "$APP_NAME"
  [[ $status -eq 0 ]]
  result=$($KETCH app info "$APP_NAME")
  echo "RECEIVED:" $result
  [[ $result =~ "Address: http://$CNAME" ]]
}

@test "cname remove" {
  run $KETCH cname remove "$CNAME" --app "$APP_NAME"
  [[ $status -eq 0 ]]
  result=$($KETCH app info "$APP_NAME")
  echo "RECEIVED:" $result
  [[ ! $result =~ "Address: http://$CNAME" ]]
}

@test "env set" {
  run $KETCH env set "$TEST_ENVVAR_KEY=$TEST_ENVVAR_VALUE" --app "$APP_NAME"
  [[ $status -eq 0 ]]
}

@test "env get" {
  result=$($KETCH env get "$TEST_ENVVAR_KEY" --app "$APP_NAME")
  echo "RECEIVED:" $result
  [[ $result =~ "$TEST_ENVVAR_VALUE" ]]
}

@test "env unset" {
  run $KETCH env unset "$TEST_ENVVAR_KEY" --app "$APP_NAME"
  [[ $status -eq 0 ]]
  result=$($KETCH env get "$TEST_ENVVAR_KEY" --app "$APP_NAME")
  echo "RECEIVED:" $result
  [[ ! $result =~ "$TEST_ENVVAR_VALUE" ]]
}

@test "app remove" {
  result=$($KETCH app remove "$APP_NAME")
  echo "RECEIVED:" $result
  [[ $result =~ "Successfully removed!" ]]
}

@test "app-istio remove" {
  result=$($KETCH app remove "$APP_NAME-istio")
  echo "RECEIVED:" $result
  [[ $result =~ "Successfully removed!" ]]
}

@test "app-nginx remove" {
  result=$($KETCH app remove "$APP_NAME-nginx")
  echo "RECEIVED:" $result
  [[ $result =~ "Successfully removed!" ]]
}

@test "app-2 remove" {
  result=$($KETCH app remove "$APP_NAME-2")
  echo "RECEIVED:" $result
  [[ $result =~ "Successfully removed!" ]]
}