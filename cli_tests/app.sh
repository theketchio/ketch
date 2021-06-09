#!/usr/bin/env bats

setup() {
  KETCH=$(pwd)/bin/ketch
  INGRESS=$(kubectl get svc traefik -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
  echo $INGRESS
}
framework="myframework"
appImage="docker.io/shipasoftware/bulletinboard:1.0"

@test "help" {
  result="$($KETCH help)"
  [[ $result =~ "For details see https://theketch.io" ]]
  [[ $result =~ "Available Commands" ]]
  [[ $result =~ "Flags" ]]
}

@test "framework create" {
  result="$($KETCH framework add $framework --ingress-service-endpoint $INGRESS --ingress-type traefik)"
  [[ $result =~ "Successfully added!" ]]
}

@test "framework list" {
  result="$($KETCH framework list)"
  echo $result
  headerRegex="NAME[ \t]+STATUS[ \t]+NAMESPACE[ \t]+INGRESS TYPE[ \t]+INGRESS CLASS NAME[ \t]+CLUSTER ISSUER[ \t]+APPS"
  dataRegex="myframework[ \t]+ketch-myframework[ \t]+traefik[ \t]+traefik"
  [[ "$result" =~ $headerRegex ]]
  [[ "$result" =~ $dataRegex ]]
}

@test "app deploy" {
  run $KETCH app deploy bulletinboard --framework $framework -i $appImage
  [ $status -eq 0 ]
}

@test "app list" {
  result="$($KETCH app list)"
  headerRegex="NAME[ \t]+FRAMEWORK[ \t]+STATE[ \t]+ADDRESSES[ \t]+BUILDER[ \t]+DESCRIPTION"
  dataRegex="bulletinboard[ \t]+myframework[ \t]+(created|running)[ \t]+http://bulletinboard.$INGRESS.shipa.cloud"
  echo $result
  [[ "$result" =~ $headerRegex ]]
  [[ "$result" =~ $dataRegex ]]
}

# TODO curl test

@test "app remove" {
  result="$($KETCH app remove bulletinboard)"
  [[ $result =~ "Successfully removed!" ]]
}

@test "framework remove" {
  result="$(echo ketch-$framework | $KETCH framework remove $framework)"
  [[ $result =~ "Framework successfully removed!" ]]
}