#!/bin/sh

DOCKER_IMAGE=hearthhub-server:0.0.5
DOCKER_DATA_VOLUME=valheim_server_data

docker run --rm -it -v "${DOCKER_DATA_VOLUME}:/root/.config/unity3d/IronGate/Valheim" -v "$PWD:/irongate" "${DOCKER_IMAGE}"
