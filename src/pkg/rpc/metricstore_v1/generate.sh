#!/bin/bash

dir_resolve()
{
    cd "$1" 2>/dev/null || return $?  # cd to desired directory; if fail, quell any error messages but return exit status
    echo "`pwd -P`" # output full, link-resolved path
}

set -e

TARGET=`dirname $0`
TARGET=`dir_resolve $TARGET`
cd $TARGET

go get github.com/golang/protobuf/{proto,protoc-gen-go}
go get github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway

tmp_dir="$(mktemp -d)/metric-store"
mkdir -p $tmp_dir

cp $GOPATH/src/github.com/cloudfoundry/metric-store-release/src/api/v1/*proto $tmp_dir

protoc \
    $tmp_dir/*.proto \
    --go_out=plugins=grpc:. \
    --proto_path=$tmp_dir \
    --grpc-gateway_out=logtostderr=true:. \
    -I$GOPATH/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis \
    -I=/usr/local/include \
    -I=$tmp_dir

rm -r $tmp_dir
