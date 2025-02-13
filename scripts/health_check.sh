#!/bin/sh
#
# NOTE IMPORTANT: Any time this file changes a new version of the valheim server needs to be built and published.
# since this file is included in the base image.
#
if grep -q "with join code" /valheim/BepInEx/config/server-logs.txt; then
    echo "Server started!"
    exit 0
else
    echo "Server still starting..."
    exit 1
fi