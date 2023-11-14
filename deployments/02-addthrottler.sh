#!/bin/bash

namespace_arr=($(kubectl get --all-namespaces po -l app=fa | awk 'NR>1 { print $1 }'))
for ns in ${namespace_arr[@]}; do
    patch=$(
        cat <<-__EOF__
{"spec":{"template":{"spec":{"containers":[{"name":"fa","image":"$1", "env":[
{"name":"FB_REPO_ENABLETHROTTLE","value":"true"}
]
}]}}}}
__EOF__
    )
    echo "patching namespace: [ $ns ] to version: [ $IMG ]"
    kubectl -n $ns patch deployment fa -p "$patch"
    kubectl -n $ns scale --replicas=0 deploy/fa
    kubectl -n $ns scale --replicas=1 deploy/fa
done