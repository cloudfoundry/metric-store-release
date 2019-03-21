#!/bin/bash

set -e

PROJECT_DIR="$(cd "$(dirname "$0")/../../../.."; pwd)"
tmp_dir="$(mktemp -d)/metric-store"
mkdir -p $tmp_dir

pushd $tmp_dir
  git clone https://github.com/grpc-ecosystem/grpc-gateway
  cp ${PROJECT_DIR}/src/api/v1/*proto .
popd

pushd ${PROJECT_DIR}/src/pkg/rpc/metricstore_v1
  protoc \
      $tmp_dir/*.proto \
      --go_out=plugins=grpc:. \
      --proto_path=$tmp_dir \
      --grpc-gateway_out=logtostderr=true:. \
      -I$tmp_dir/grpc-gateway/third_party/googleapis \
      -I=/usr/local/include \
      -I=$tmp_dir
popd

rm -rf $tmp_dir
