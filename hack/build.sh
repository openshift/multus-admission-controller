#!/usr/bin/env bash
set -e

DEST_DIR="bin"

if [ ! -d ${DEST_DIR} ]; then
        mkdir ${DEST_DIR}
fi

# this if... will be removed when gomodules goes default
if [ "$GO111MODULE" == "off" ]; then
        echo "Building admission controller without go module"
        echo "Warning: this will be deprecated in near future so please use go modules!"

	ORG_PATH="github.com/k8snetworkplumbingwg"
	REPO_PATH="${ORG_PATH}/net-attach-def-admission-controller"

	if [ ! -h gopath/src/${REPO_PATH} ]; then
		mkdir -p gopath/src/${ORG_PATH}
		ln -s ../../../.. gopath/src/${REPO_PATH} || exit 255
	fi

	#cp -a vendor/* gopath/src/
        export GO15VENDOREXPERIMENT=1
        export GOBIN=${PWD}/bin
        export GOPATH=${PWD}/gopath

	# go install ./...
	go build -o ./bin/webhook ${REPO_PATH}/cmd/webhook
else
        # build with go modules
        export GO111MODULE=on

        echo "Building admission controller"
        go build -o ${DEST_DIR}/webhook -tags no_openssl -ldflags "${LDFLAGS}" "$@" ./cmd/webhook
fi
# go install ./...
