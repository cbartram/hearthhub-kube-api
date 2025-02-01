#!/bin/sh
if grep -q "Game server connected" /valheim/output.log; then
    echo "Server started!"
    exit 0
else
    echo "Server still starting..."
    exit 1
fi