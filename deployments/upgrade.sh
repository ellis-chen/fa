#!/bin/bash
# update the tenant level fa deployment
set -ex

function log_info {
    local DATE=$(date "+%Y-%m-%d %H:%M:%S[%Z]")
    echo -e "\033[35;1m${DATE}\033[35;1m: \033[36;1m[info    ]: $1\033[0m"
}

function log_err() {
    local DATE=$(date "+%Y-%m-%d %H:%M:%S[%Z]")
    echo -e "\033[35;1m${DATE}\033[35;1m: \033[31;1m[error   ]: $1\033[0m"
}

function log_warn() {
    local DATE=$(date "+%Y-%m-%d %H:%M:%S[%Z]")
    echo -e "\033[35;1m${DATE}\033[35;1m: \033[1;33m[warn    ]: $1\033[0m"
}

function log_suc() {
    local DATE=$(date "+%Y-%m-%d %H:%M:%S[%Z]")
    echo -e "\033[35;1m${DATE}\033[35;1m: \033[32;1m[success ]: $1\033[0m"
}

function usage() {
    echo "Usage: $0 [-v <version of fa>] " 1>&2
    exit 1
}
function main() {
    # if [ $# -le 1 ]; then
    #     usage
    # fi

    # prepare the env,
    # Read in command line options
    while (($# > 0)); do
        opt="$1"
        case $opt in
        "-v" | "--version")
            version="$2"
            if [[ $version == "" ]]; then
                version=latest
            fi
            shift
            ;;
        "-n" | "--namespace")
            namespace="$2"
            shift
            ;;
        "-r")
            rollout
            exit 0
            ;;
        "-s" | "--sftp")
            sftp_version="$2"
            if [[ $sftp_version == "" ]]; then
                sftp_version=latest
            fi
            shift
            ;;
        "-h" | "--help")
            usage
            ;;
        --) # End of all options
            shift
            break
            ;;
        *)
            echo "unknown arg $opt"
            usage
            ;;
        esac
        shift
    done

    if [ -z "$version" ]; then
        upgrade
    else
        upgradeByVersion $version
    fi

    # if [[ $version == "" ]]; then
    #     version=latest
    # fi

    # if [ -n "$sftp_version" ]; then
    #     upgradeSvc2Headless
    #     upgradeSftpByVersion $sftp_version
    # fi

    # if [ -n "$namespace" ]; then
    #     upgradeSingleSftpByVersion $namespace $version
    # fi
}

function upgrade() {
    namespace_arr=($(kubectl get --all-namespaces po -l app=fa | awk 'NR>1 { print $1 }'))
    for ns in ${namespace_arr[@]}; do
        log_info "upgrading namespace: [ $ns ]"
        kubectl -n $ns scale --replicas=0 deploy/fa
        kubectl -n $ns scale --replicas=1 deploy/fa
    done
}

function rollout() {
    namespace_arr=($(kubectl get --all-namespaces po -l app=fa | awk 'NR>1 { print $1 }'))
    for ns in ${namespace_arr[@]}; do
        log_info "rollout namespace: [ $ns ]"
        kubectl -n $ns rollout undo deploy/fa
    done
}

function upgradeByVersion() {
    namespace_arr=($(kubectl get --all-namespaces po -l app=fa | awk 'NR>1 { print $1 }'))
    for ns in ${namespace_arr[@]}; do
        patch=$(
            cat <<-__EOF__
{"spec":{"template":{"spec":{"containers":[{"name":"fa","image":"r.ellis-chentech.com:5000/fa:$1", "env":[{"name":"FA_CLOUD_ACCOUNT_NAME","value":"test"}]}]}}}}
__EOF__
        )
        log_info "patching namespace: [ $ns ] to version: [ $1 ]"
        eval kubectl -n $ns patch deployment fa -p "'"$patch"'"
    done
}

function upgradeSvc2Headless() {
    namespace_arr=($(kubectl get --all-namespaces po -l app=fa | awk 'NR>1 { print $1 }'))

    for ns in ${namespace_arr[@]}; do
        #         patch=$(
        #             cat <<-__EOF__
        # {"spec":{"clusterIP": "None", "ports":[{"port":22, "protocol":"TCP", "targetPort":22, "name":"sftp"}]}}
        # __EOF__
        #         )
        kubectl -n $ns delete svc fa
        log_info "patching namespace: [ $ns ] to version: [ $1 ]"
        kubectl -n $ns apply -f ./patch-svc.yaml
    done
}

function upgradeSftpByVersion() {
    namespace_arr=($(kubectl get --all-namespaces po -l app=fa | awk 'NR>1 { print $1 }'))
    for ns in ${namespace_arr[@]}; do
        patch=$(
            cat <<-__EOF__
{"spec":{"template":{"spec":{"containers":[{"name":"sftp","image":"r.ellis-chentech.com:5000/fs-sftp-ldap","imagePullPolicy":"Always","env":[{"name":"LDAP_URI","value":"ldap://ellis-chen-demo-global-ldap-644ec931cbf43942.elb.cn-northwest-1.amazonaws.com.cn:389"},{"name":"LDAP_BASE","value":"dc=ellis-chentech,dc=demo"},{"name":"LDAP_BIND_USER","value":"cn=admin,dc=ellis-chentech,dc=demo"},{"name":"LDAP_BIND_PWD","value":"ellis-chen112233"}],"ports":[{"containerPort":22,"name":"tcp"}],"volumeMounts":[{"mountPath":"/ellis-chen","name":"data"}]}]}}}}
__EOF__
        )
        log_info "patching namespace: [ $ns ] to version: [ $1 ]"
        eval kubectl -n $ns patch deployment fa -p "'"$patch"'"
    done
}

function upgradeSingleSftpByVersion() {
    patch=$(
        cat <<-__EOF__
{"spec":{"template":{"spec":{"containers":[{"name":"sftp","image":"r.ellis-chentech.com:5000/fs-sftp-ldap:$2","imagePullPolicy":"Always","env":[{"name":"LDAP_URI","value":"ldap://ellis-chen-demo-ldap-5a9e8e8974d0be19.elb.cn-northwest-1.amazonaws.com.cn:389"},{"name":"LDAP_BASE","value":"dc=ellis-chentech,dc=demo"},{"name":"LDAP_BIND_USER","value":"cn=admin,dc=ellis-chentech,dc=demo"},{"name":"LDAP_BIND_PWD","value":"ellis-chen112233"}],"ports":[{"containerPort":22,"name":"tcp"}],"volumeMounts":[{"mountPath":"/ellis-chen","name":"data"}]}]}}}}
__EOF__
    )
    log_info "patching namespace: [ $1 ] to version: [ $2 ]"
    eval kubectl -n $1 patch deployment fa -p "'"$patch"'"
}

function addBackup() {
    namespace_arr=($(kubectl get --all-namespaces po -l app=fa | awk 'NR>1 { print $1 }'))
    for ns in ${namespace_arr[@]}; do
        patch=$(
            cat <<-__EOF__
{"spec":{"template":{"spec":{"containers":[{"name":"fa","image":"r.ellis-chentech.com:5000/fa:local", "env":[
    {"name":"FB_LOC_VENDOR","value":"baidu"},
    {"name":"FB_LOC_REGION","value":"bj"},
    {"name":"FB_LOC_NS","value":"-"},
    {"name":"FB_LOC_PREFIX","value":"$ns"},
    {"name":"FB_CRED_KEYPAIR_KEY","value":"ak"},
    {"name":"FB_CRED_KEYPAIR_SECRET","value":"sk"}
    ]
}]}}}}
__EOF__
        )
        echo "patching namespace: [ $ns ] to version: [ $1 ]"
        kubectl -n $ns patch deployment fa -p "$patch"
        kubectl -n $ns scale --replicas=0 deploy/fa
        kubectl -n $ns scale --replicas=1 deploy/fa
    done
}


# 1432300453542105088
main $@
