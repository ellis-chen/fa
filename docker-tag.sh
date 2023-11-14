#!/bin/bash

TAG_TYPE="long"

while getopts ":ls" options; do
    case ${options} in
    l)
        TAG_TYPE="long"
        ;;
    s)
        TAG_TYPE="short"
        ;;
    :)
        echo "ERROR: Invalid option: ${OPTARG} requires an argument" 1>&2
        exit 1
        ;;
    \?)
        echo "ERROR: Invalid option: ${OPTARG}" 1>&2
        exit 1
        ;;
    esac
done
shift $((OPTIND - 1))

# generate tag by branch name
# if it's from a release branch append build id

# workaround gitlab runner
if [ -n "${CI_COMMIT_REF_NAME}" ]; then
    BRANCH=${CI_COMMIT_REF_NAME}
else
    BRANCH=$(git rev-parse --abbrev-ref HEAD)
fi

if echo ${BRANCH} | grep -e "^release/" &>/dev/null; then
    TAG=$(echo ${BRANCH} | sed -e 's#^release/##g' -e 's#/#.#g')
else
    TAG=$(echo ${BRANCH} | sed -e 's#/#.#g')
fi

if [ $TAG_TYPE == "long" ]; then
    TAG="$TAG-${CI_PIPELINE_ID:-dev}"
fi

echo $TAG
