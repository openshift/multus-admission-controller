#!/usr/bin/env bash
set -e

ORG_PATH="github.com/K8sNetworkPlumbingWG"
REPO_PATH="${ORG_PATH}/net-attach-def-admission-controller"

if [ ! -h gopath/src/${REPO_PATH} ]; then
	mkdir -p gopath/src/${ORG_PATH}
	ln -s ../../../.. gopath/src/${REPO_PATH} || exit 255
fi

cp -a vendor/* gopath/src/

export GOBIN=${PWD}/bin
export GOPATH=${PWD}/gopath

echo "Building admission controller"
# go install ${REPO_PATH}/...
mkdir -p bin
workdir=$(pwd)
cd gopath/src/${REPO_PATH}
# go install ./...
go build -o ./bin/installer ${REPO_PATH}/cmd/installer
go build -o ./bin/webhook ${REPO_PATH}/cmd/webhook
chmod +x ./bin/installer
chmod +x ./bin/webhook
cd $workdir
# go install ./...


