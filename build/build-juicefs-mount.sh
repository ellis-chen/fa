#!/bin/bash
VERSION=v1.0.2
docker build -t hub.ellis-chentech.com/tools/juicefs-mount:$VERSION -f Dockerfile.juicefs.mount ..
docker push hub.ellis-chentech.com/tools/juicefs-mount:$VERSION