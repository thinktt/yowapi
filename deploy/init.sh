#!/usr/bin/env bash
set -euo pipefail

docker network create yow >/dev/null 2>&1 || true
docker network create mongonet >/dev/null 2>&1 || true
docker network create proxynet >/dev/null 2>&1 || true
docker volume create mongodb >/dev/null

echo "Initialized Docker networks: yow, mongonet, proxynet"
echo "Initialized Docker volume: mongodb"
