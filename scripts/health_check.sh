#!/bin/sh
if grep -q "with join code" /valheim/BepInEx/config/server-logs.txt; then
    echo "Server started!"
    exit 0
else
    echo "Server still starting..."
    exit 1
fi