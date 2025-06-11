#!/usr/bin/env bash
set -fb

THISDIR=$(cd $(dirname $0) ; pwd)
ScanCliArgs="${@:3}"


if [ $1 != '--provider' ] || [ $3 != '--image' ]; then
   echo "Missing required options. \nExample usage:  \"./scan.container.registry.sh --provider google --image us.gcr.io/eng-dev/containerbuilder/sample --host xxxx --username xxxx\"."
   exit 2
fi

if [ -z "$(which docker)" ]; then
   echo "Docker is required to be in your PATH and it cannot be found."
   exit 2
fi

case $2 in
    google)
        echo "Executing scan.google.sh....." $ScanCliArgs
        if [ -z "$(which gcloud)" ]; then
           echo "gcloud is required to be in your PATH and it cannot be found."
           exit 2
        fi

        if [[ $4 != *"gcr.io"* ]]; then
           echo "Not a valid image name since it does not contain GCR.IO sub string"
           exit 2
        fi
        gcloud docker -- pull $4 
        ;;
    *)
        echo "Provider not supported. Provoder option values are google only."
        exit 2
        ;;
esac
echo "Executing scan.docker.sh....." $ScanCliArgs

"${THISDIR}"/scan.docker.sh "${@:3}"

docker rmi -f $4
