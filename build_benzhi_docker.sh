#!/usr/bin/env sh
set -eu
IMAGE_NAME="${1:-ygw-go-09-06:benzhi}"
docker buildx build --load --platform "${TARGET_PLATFORM:-linux/amd64}" -f benzhi.Dockerfile -t "$IMAGE_NAME" .
