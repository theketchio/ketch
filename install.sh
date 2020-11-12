#!/usr/bin/env bash

RELEASE_DOWNLOAD_URL="https://github.com/shipa-corp/ketch/releases"

: "${INSTALL_DIR:="/usr/local/bin"}"


# Get Ketch tag requested by the user from ENV variable
hasUserProvidedTAG() {
  [[ -n "$TAG" ]]
}

getPlatform() {
  set -eu

  UNAME=$(uname)
  if [ "$UNAME" != "Linux" ] && [ "$UNAME" != "Darwin" ]; then
    echo "Sorry, OS not supported: ${UNAME}. Download binary from ${RELEASE_DOWNLOAD_URL}"
    exit 1
  fi

  case "${UNAME}" in
  Darwin)
    OSX_ARCH=$(uname -m)
    if [ "${OSX_ARCH}" = "x86_64" ]; then
      PLATFORM="darwin-amd64"
    else
      echo "Sorry, architecture not supported: ${OSX_ARCH}. Download binary from ${RELEASE_DOWNLOAD_URL}"
      exit 1
    fi

    ;;

  Linux)
    LINUX_ARCH=$(uname -m)
    if [ "${LINUX_ARCH}" = "x86_64" ]; then
      PLATFORM="linux-amd64"
    else
      echo "Sorry, architecture not supported: ${LINUX_ARCH}. Download binary from ${RELEASE_DOWNLOAD_URL}"
      exit 1
    fi
    ;;
  esac
}

getLatestTag() {
  TAG=$(curl -s https://api.github.com/repos/shipa-corp/ketch/releases/latest | grep -Eo '"tag_name":.*[^\\]",' | head -n 1 | sed 's/[," ]//g' | cut -d ':' -f 2)
}

install() {
  DOWNLOAD_URL="${RELEASE_DOWNLOAD_URL}/download/$TAG/ketch-$PLATFORM"
  DEST=${DEST:-${INSTALL_DIR}/ketch}

  if [ -z "$TAG" ]; then
    echo "Error requesting. Download binary from ${RELEASE_DOWNLOAD_URL}"
    exit 1
  else
    echo "Downloading Ketch binary from ${DOWNLOAD_URL} to $DEST"
    if curl -sL "${DOWNLOAD_URL}" -o "$DEST"; then
      chmod +x "$DEST"
      echo "Ketch client installation was successful"
    else
      echo "Installation failed. You may need elevated permissions."
    fi
  fi
}

hasUserProvidedTAG || getLatestTag
getPlatform
install
