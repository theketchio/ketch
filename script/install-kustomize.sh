#!/bin/bash


set -euxo pipefail

curr_dir=$(pwd)

tmp_dir=$(mktemp -d -t kustomize-XXXXXXXXXX)
cd "${tmp_dir}"

git clone ssh://git@github.com/kubernetes-sigs/kustomize
cd kustomize
git checkout v3.3.1
cd kustomize
go install .

cd "${curr_dir}"

rm -rf "${tmp_dir}" 
