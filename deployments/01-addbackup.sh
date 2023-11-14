#!/bin/bash

function usage() {
    echo "Usage: $0 -i <fa img> -a <access key> -s <secret key> -r <region> [-v <aws|baidu|oracle>] [-n <oracle namespace>] [-b <bucket name>] [-h]" 1>&2
    exit 1
}


while getopts ":i:a:s:hr:v:n:b:" opt; do
    case ${opt} in
    i)
        IMG=${OPTARG}
        ;;
    a)
        AK=${OPTARG}
        ;;
    s)
        SK=${OPTARG}
        ;;
    r)
        REGION=${OPTARG}
        ;;
    v)
        VENDOR=${OPTARG}
        [[ ! $VENDOR =~ aws|baidu|oracle ]] && {
            echo "Incorrect vendor provided: [ $VENDOR ]"
            exit 1
        }
        ;;
    h)
        usage
        ;;
    n)
        NAMESPACE=${OPTARG}
        ;;
    b)
        BUCKET=${OPTARG}
        ;;
    :)
        echo "ERROR: Invalid option: ${opt} requires an argument" >&2
        exit 1
        ;;
    *)
        usage
        ;;
    esac
done
shift $((OPTIND - 1))

if [ -z "${IMG}" ]; then
    echo "ERROR: missing fa img"
    usage
fi

if [ -z "${AK}" ]; then
    echo "ERROR: missing accesskey"
    usage
fi

if [ -z "${SK}" ]; then
    echo "ERROR: missing secretkey"
    usage
fi

if [ -z "${VENDOR}" ]; then
    echo "ERROR: missing vendor."
    usage
fi

if [ -z "${REGION}" ]; then
    echo "ERROR: missing region"
    usage
fi

if [ "${VENDOR}" == "oracle" ] && [ -z "${NAMESPACE}" ]; then
    echo "ERROR: missing namespace in oracle scenario"
    usage
fi

if [ -z "${BUCKET}" ]; then
    BUCKET="kopia-backup-center"
fi


namespace_arr=($(kubectl get --all-namespaces po -l app=fa | awk 'NR>1 { print $1 }'))
for ns in ${namespace_arr[@]}; do
    patch=$(
        cat <<-__EOF__
{"spec":{"template":{"spec":{"containers":[{"name":"fa","image":"$IMG", "env":[
{"name":"FB_LOC_BUCKET","value":"${BUCKET}"},
{"name":"FB_LOC_VENDOR","value":"$VENDOR"},
{"name":"FB_LOC_REGION","value":"$REGION"},
{"name":"FB_LOC_NS","value":"${NAMESPACE:--}"},
{"name":"FB_LOC_PREFIX","value":"$ns"},
{"name":"FB_CRED_KEYPAIR_KEY","value":"${AK}"},
{"name":"FB_CRED_KEYPAIR_SECRET","value":"${SK}"}
]
}]}}}}
__EOF__
    )
    echo "patching namespace: [ $ns ] to version: [ $IMG ]"
    kubectl -n $ns patch deployment fa -p "$patch"
    kubectl -n $ns scale --replicas=0 deploy/fa
    kubectl -n $ns scale --replicas=1 deploy/fa
done