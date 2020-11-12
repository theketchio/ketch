#!/bin/bash


set -euo pipefail

kustomize_version="${KUSTOMIZE_VERSION:-v3.3.1}"

if command -v kustomize > /dev/null 
then
    echo "kustomize already installed, installation skipped"
    exit
fi

echo "Installing kustomize ${kustomize_version} from source"

curr_dir=$(pwd)

tmp_dir=$(mktemp -d -t kustomize-XXXXXXXXXX)
cd "${tmp_dir}"

git clone ssh://git@github.com/kubernetes-sigs/kustomize
cd kustomize
git checkout "${kustomize_version}"
cd kustomize
go install .

cd "${curr_dir}"

rm -rf "${tmp_dir}" 
