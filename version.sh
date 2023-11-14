#!/bin/bash

# workaround gitlab runner
if [ -n "${CI_COMMIT_REF_NAME}" ]; then
    BRANCH=${CI_COMMIT_REF_NAME}
else
    BRANCH=$(git rev-parse --abbrev-ref HEAD)
fi

# find version by tag, if not found, use brand name and commit id
VERSION=$(git describe --tags --always --dirty)
if git describe >/dev/null 2>&1; then
    echo $VERSION
else
    echo ${BRANCH}-$VERSION
fi
