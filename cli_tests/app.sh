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
  INGRESS_NGINX=$(kubectl get svc ingress-nginx-controller -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
  INGRESS_ISTIO=$(kubectl get svc istio-ingressgateway -n istio-system -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
  FRAMEWORK="myframework"
  APP_IMAGE="gcr.io/shipa-ci/sample-go-app:latest"
  APP_NAME="sample-app"
  CNAME="my-cname.com"
  TEST_ENVVAR_KEY="FOO"
  TEST_ENVVAR_VALUE="BAR"
}

teardown() {
  rm -f app.yaml
  rm -f framework.yaml
}

@test "help" {
  result="$($KETCH help)"
  echo "RECEIVED:" $result
  [[ $result =~ "For details see https://theketch.io" ]]
  [[ $result =~ "Available Commands" ]]
  [[ $result =~ "Flags" ]]
}

@test "framework add" {
  result=$($KETCH framework add "$FRAMEWORK" --ingress-service-endpoint "$INGRESS_TRAEFIK" --ingress-type "traefik")
  echo "RECEIVED:" $result
  [[ $result =~ "Successfully added!" ]]
}

@test "framework add istio" {
  result=$($KETCH framework add "$FRAMEWORK-istio" --ingress-service-endpoint "$INGRESS_ISTIO" --ingress-type "istio")
  echo "RECEIVED:" $result
  [[ $result =~ "Successfully added!" ]]
}

@test "framework add nginx" {
  result=$($KETCH framework add "$FRAMEWORK-nginx" --ingress-service-endpoint "$INGRESS_NGINX" --ingress-type "nginx")
  echo "RECEIVED:" $result
  [[ $result =~ "Successfully added!" ]]
}

@test "framework add error" {
  run $KETCH framework add "$FRAMEWORK" --ingress-service-endpoint "$INGRESS_TRAEFIK" --ingress-type "traefik"
  [[ $status -eq 1 ]]
  [[ $output =~ "\"$FRAMEWORK\" already exists" ]]
}

@test "framework add with yaml file" {
  cat << EOF > framework.yaml
name: "$FRAMEWORK-2"
app-quota-limit: 1
ingressController:
  className: traefik
  serviceEndpoint: 10.10.20.30
  type: traefik
EOF
  result=$($KETCH framework add framework.yaml)
  echo "RECEIVED:" $result
  [[ $result =~ "Successfully added!" ]]

  # assert add
  result=$($KETCH framework list)
  dataRegex="$FRAMEWORK-2[ \t]+ketch-$FRAMEWORK-2[ \t]+traefik[ \t]+traefik"
  echo "RECEIVED:" $result
  [[ $result =~ $dataRegex ]]
}

@test "framework list" {
  result=$($KETCH framework list)
  headerRegex="NAME[ \t]+STATUS[ \t]+NAMESPACE[ \t]+INGRESS TYPE[ \t]+INGRESS CLASS NAME[ \t]+CLUSTER ISSUER[ \t]+APPS"
  dataRegexTraefik="$FRAMEWORK[ \t]+ketch-$FRAMEWORK[ \t]+traefik[ \t]+traefik"
  dataRegexNginx="$FRAMEWORK-nginx[ \t]+ketch-$FRAMEWORK-nginx[ \t]+nginx[ \t]+nginx"
  echo "RECEIVED:" $result
  [[ $result =~ $headerRegex ]]
  [[ $result =~ $dataRegexTraefik ]]
  [[ $result =~ $dataRegexNginx ]]
}

@test "framework update" {
  result=$($KETCH framework update "$FRAMEWORK" --app-quota-limit 2)
  echo "RECEIVED:" $result
  [[ $result =~ "Successfully updated!" ]]
}

@test "framework export" {
  run $KETCH framework export "$FRAMEWORK" -f framework.yaml
  result=$(cat framework.yaml)
  echo "RECEIVED:" $result
  [[ $result =~ "name: $FRAMEWORK" ]]
  [[ $result =~ "namespace: ketch-$FRAMEWORK" ]]
  [[ $result =~ "appQuotaLimit: 2" ]]
  rm -f framework.yaml
}

@test "framework update with yaml file" {
  cat << EOF > framework.yaml
name: "$FRAMEWORK-2"
app-quota-limit: 2
ingressController:
  className: istio
  serviceEndpoint: 10.10.20.30
  type: istio
EOF
  result=$($KETCH framework update framework.yaml)
  echo "RECEIVED:" $result
  [[ $result =~ "Successfully updated!" ]]
  # assert update
  result=$($KETCH framework list)
  dataRegex="$FRAMEWORK-2[ \t]+ketch-$FRAMEWORK-2[ \t]+istio[ \t]+istio"
  echo "RECEIVED:" $result
  [[ $result =~ $dataRegex ]]
}

@test "app deploy" {
  run $KETCH app deploy "$APP_NAME" --framework "$FRAMEWORK" -i "$APP_IMAGE"
  [[ $status -eq 0 ]]
}

@test "app deploy istio" {
  run $KETCH app deploy "$APP_NAME-istio" --framework "$FRAMEWORK-istio" -i "$APP_IMAGE"
  [[ $status -eq 0 ]]
}

@test "app deploy nginx" {
  run $KETCH app deploy "$APP_NAME-nginx" --framework "$FRAMEWORK-nginx" -i "$APP_IMAGE"
  [[ $status -eq 0 ]]
}

@test "app deploy with yaml file" {
  cat << EOF > app.yaml
name: "$APP_NAME-2"
version: v1
type: Application
image: "$APP_IMAGE"
framework: "$FRAMEWORK"
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
  [[ $result =~ "Framework: $FRAMEWORK" ]]
  [[ $result =~ "Version: v1" ]]
  [[ $result =~ "Description: cli test app" ]]
}

@test "app unit set" {
 run $KETCH app deploy "$APP_NAME" --framework "$FRAMEWORK" -i "$APP_IMAGE" --units 3
 [[ $status -eq 0 ]]
  result=$(kubectl describe apps $APP_NAME)
  echo "RECEIVED:" $result
 [[ $result =~ "Units:  3" ]] # note two spaces
}

@test "app list" {
  result=$($KETCH app list)
  headerRegex="NAME[ \t]+FRAMEWORK[ \t]+STATE[ \t]+ADDRESSES[ \t]+BUILDER[ \t]+DESCRIPTION"
  dataRegex="$APP_NAME[ \t]+$FRAMEWORK[ \t]+(created|1 running)"
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
  [[ $result =~ "framework: $FRAMEWORK" ]]
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

@test "framework remove" {
  result=$(echo "ketch-$FRAMEWORK" | $KETCH framework remove "$FRAMEWORK")
  echo "RECEIVED:" $result
  [[ $result =~ "Framework successfully removed!" ]]
}

@test "framework-istio remove" {
  result=$(echo "ketch-$FRAMEWORK-istio" | $KETCH framework remove "$FRAMEWORK-istio")
  echo "RECEIVED:" $result
  [[ $result =~ "Framework successfully removed!" ]]
}

@test "framework-nginx remove" {
  result=$(echo "ketch-$FRAMEWORK-nginx" | $KETCH framework remove "$FRAMEWORK-nginx")
  echo "RECEIVED:" $result
  [[ $result =~ "Framework successfully removed!" ]]
}

@test "framework-2 remove" {
  result=$(echo "ketch-$FRAMEWORK-2" | $KETCH framework remove "$FRAMEWORK-2")
  echo "RECEIVED:" $result
  [[ $result =~ "Framework successfully removed!" ]]
}
