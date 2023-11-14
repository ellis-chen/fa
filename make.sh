#!/bin/bash
set -ex
HUB_URL=${HUB_URL:-hub.ellis-chentech.com}
REGISTERY=${HUB_URL}/cce

LONG_TAG=$(bash docker-tag.sh -l)
SHORT_TAG=$(bash docker-tag.sh -s)
VERSION=$(bash version.sh)
IMAGE_NAME=fa

# ellis-chen common meta
COMMIT_ID=$(git rev-parse HEAD)
PIPELINE_ID=${CI_PIPELINE_ID:-dev}
if [ -n "${CI_COMMIT_REF_NAME}" ]; then
    REF_NAME=${CI_COMMIT_REF_NAME}
else
    REF_NAME=$(git rev-parse --abbrev-ref HEAD)
fi
BUILT_TIME=$(date +%s)

export DOCKER_BUILDKIT=1
# either ENABLE_JUICEFS flag is set or in a semver version
if [[ -n $ENABLE_JUICEFS || $VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]{1,2}$ ]]; then
    docker build ${BUILD_ARGS} \
        -t ${REGISTERY}/${IMAGE_NAME}:${LONG_TAG} \
        --build-arg COMMIT_ID=${COMMIT_ID} \
        --build-arg PIPELINE_ID=${PIPELINE_ID} \
        --build-arg REF_NAME=${REF_NAME} \
        --build-arg BUILT_TIME=${BUILT_TIME} \
        -f build/Dockerfile.full.juicefs .
elif [ -f bin/$IMAGE_NAME ]; then
    docker build ${BUILD_ARGS} \
        -t ${REGISTERY}/${IMAGE_NAME}:${LONG_TAG} \
        --build-arg COMMIT_ID=${COMMIT_ID} \
        --build-arg PIPELINE_ID=${PIPELINE_ID} \
        --build-arg REF_NAME=${REF_NAME} \
        --build-arg BUILT_TIME=${BUILT_TIME} \
        -f build/Dockerfile.lite .
else
    docker build ${BUILD_ARGS} \
        -t ${REGISTERY}/${IMAGE_NAME}:${LONG_TAG} \
        --build-arg COMMIT_ID=${COMMIT_ID} \
        --build-arg PIPELINE_ID=${PIPELINE_ID} \
        --build-arg REF_NAME=${REF_NAME} \
        --build-arg BUILT_TIME=${BUILT_TIME} \
        -f build/Dockerfile.full .
fi


docker tag ${REGISTERY}/${IMAGE_NAME}:${LONG_TAG} ${REGISTERY}/${IMAGE_NAME}:${SHORT_TAG}

docker push ${REGISTERY}/${IMAGE_NAME}:${LONG_TAG}
docker push ${REGISTERY}/${IMAGE_NAME}:${SHORT_TAG}

if [[ $SHORT_TAG == master ]]; then
    docker tag ${REGISTERY}/${IMAGE_NAME}:${LONG_TAG} ${REGISTERY}/${IMAGE_NAME}:latest
    docker tag ${REGISTERY}/${IMAGE_NAME}:${LONG_TAG} ${REGISTERY}/${IMAGE_NAME}:local
    docker tag ${REGISTERY}/${IMAGE_NAME}:${LONG_TAG} r.ellis-chentech.com:5000/${IMAGE_NAME}:local
    docker push ${REGISTERY}/${IMAGE_NAME}:latest
    # local tag for backward compatibility
    docker push ${REGISTERY}/${IMAGE_NAME}:local
    docker push r.ellis-chentech.com:5000/${IMAGE_NAME}:local

    # push to infra branch too
    docker tag ${REGISTERY}/${IMAGE_NAME}:${LONG_TAG} ${HUB_URL}/infra/${IMAGE_NAME}:${LONG_TAG}
    docker push ${HUB_URL}/infra/${IMAGE_NAME}:${LONG_TAG}
    docker tag ${REGISTERY}/${IMAGE_NAME}:${LONG_TAG} ${HUB_URL}/infra/${IMAGE_NAME}:latest
    docker push ${HUB_URL}/infra/${IMAGE_NAME}:latest
    docker tag ${REGISTERY}/${IMAGE_NAME}:${LONG_TAG} ${HUB_URL}/cc3.0/${IMAGE_NAME}:${LONG_TAG}
    docker push ${HUB_URL}/cc3.0/${IMAGE_NAME}:${LONG_TAG}
    docker tag ${REGISTERY}/${IMAGE_NAME}:${LONG_TAG} ${HUB_URL}/cc3.0/${IMAGE_NAME}:latest
    docker push ${HUB_URL}/cc3.0/${IMAGE_NAME}:latest
fi

if [[ $VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]{1,2}$ ]]; then # semir version, we push to cc repo
    docker tag ${REGISTERY}/${IMAGE_NAME}:${LONG_TAG} ${HUB_URL}/infra/${IMAGE_NAME}:${VERSION}
    docker push ${HUB_URL}/infra/${IMAGE_NAME}:${VERSION}
    docker tag ${REGISTERY}/${IMAGE_NAME}:${LONG_TAG} ${HUB_URL}/cc3.0/${IMAGE_NAME}:${VERSION}
    docker push ${HUB_URL}/cc3.0/${IMAGE_NAME}:${VERSION}
fi