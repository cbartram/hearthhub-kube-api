#!/bin/sh

DOCKER_IMAGE=cbartram/hearthhub:0.0.6
DOCKER_DATA_VOLUME=valheim_server_data

docker run --rm -it -v "${DOCKER_DATA_VOLUME}:/root/.config/unity3d/IronGate/Valheim" -v "$PWD:/irongate" "${DOCKER_IMAGE}"
