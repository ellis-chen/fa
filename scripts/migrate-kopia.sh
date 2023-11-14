#!/bin/bash

function check_kopia() {
    need_cmd rclone
    need_cmd kopia

    unset KOPIA_PASSWORD
    unset KOPIA_CONFIG_PATH

    cat > rclone.conf <<EOF
[s3]
type = s3
provider = $PROVIDER
region = $REGION
env_auth = false
access_key_id = $AK
secret_access_key = $SK
endpoint = $EP
EOF

    tenants=($(rclone --config rclone.conf lsd s3:$BUCKET | awk '{print $5}'))
    for tenant in ${tenants[@]}; do
        log_info "checking tenant backup repository:  ${tenant}"
        if [ $(kopia repository connect s3 --bucket $BUCKET --access-key $AK --secret-access-key $SK --region $REGION --endpoint $EP --prefix $tenant/ -p $OLD_PWD --config-file `pwd`/$tenant.config >/dev/null 2>&1 ; echo $?) = 0 ]; then
            log_warn "kopia repository password outdate detected, tenant [$tenant], change default password"
            log_info "kopia repository connect s3 --bucket $BUCKET --access-key $AK --secret-access-key $SK --region $REGION --endpoint $EP --prefix $tenant/ -p mypass --config-file `pwd`/$tenant.config"
            log_info "kopia repository change-password -p $OLD_PWD --new-password $PRESENT_PWD --config-file `pwd`/$tenant.config"
            kopia repository change-password -p $OLD_PWD --new-password $PRESENT_PWD --config-file `pwd`/$tenant.config
            log_info "kopia repository disconnect -p $PRESENT_PWD --config-file `pwd`/$tenant.config"
            kopia repository disconnect -p $PRESENT_PWD --config-file `pwd`/$tenant.config
        else 
            log_suc "tenant [$tenant] ok, no need to reset password"
        fi  
    done

}

function usage() {
    echo "Usage: $0 -a <access key> -s <secret key> -b <bucket> [-r <region>] [-t <vendor-type(aws|baidu|oracle)>] [-n <namespace(required by oracle)>] [-o <old-password>] [-p <current-password>] " 1>&2
    exit 1
}

function check_cmd() {
    command -v "$1" >/dev/null 2>&1
}

need_cmd() {
    if ! check_cmd "$1"; then
        if test -z "$2"; then
            log_err "need '$1' (command not found)"
            err "need '$1' (command not found)"
        else
            log_err "$2"
            err "$2"
        fi
    fi

}

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

function install_rclone() {
    log_info "installing rclone"

    if check_cmd rclone; then
        return 0
    elif check_cmd yum; then
        sudo yum install rclone -y
    elif check_cmd apt; then
        sudo apt install rclone -y
    fi
}

function install_kopia() {
    log_info "installing kopia"

    if check_cmd kopia; then
        return 0
    elif check_cmd yum; then
        rpm --import https://kopia.io/signing-key
        cat <<EOF | sudo tee /etc/yum.repos.d/kopia.repo
[Kopia]
name=Kopia
baseurl=http://packages.kopia.io/rpm/stable/\$basearch/
gpgcheck=1
enabled=1
gpgkey=https://kopia.io/signing-key
EOF
        sudo yum install kopia -y
    elif check_cmd apt; then
        curl -s https://kopia.io/signing-key | sudo gpg --dearmor -o /usr/share/keyrings/kopia-keyring.gpg
        echo "deb [signed-by=/usr/share/keyrings/kopia-keyring.gpg] http://packages.kopia.io/apt/ stable main" | sudo tee /etc/apt/sources.list.d/kopia.list
        sudo apt update && sudo apt install kopia -y
    fi
}

function main() {
    while getopts "a:s:t::n::b:hr::p::o::" opt; do
        case ${opt} in
        a)
            export AK=${OPTARG}
            ;;
        s)
            export SK=${OPTARG}
            ;;
        t)
            export OSS_TYPE=${OPTARG}
            ;;
        n)
            export NS=${OPTARG}
            ;;
        b)
            export BUCKET=${OPTARG}
            ;;
        r)
            export REGION=${OPTARG}
            ;;
        o)
            export OLD_PWD=${OPTARG}
            ;;
        p)
            export PRESENT_PWD=${OPTARG}
            ;;
        h)
            usage
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

    if [ -z "${AK}" ]; then
        echo "ERROR: missing s3 accesskey"
        usage
    fi

    if [ -z "${SK}" ]; then
        echo "ERROR: missing s3 secretkey"
        usage
    fi

    if [ -z "${BUCKET}" ]; then
        echo "ERROR: missing s3 bucket"
        usage
    fi

    if [ -z "${REGION}" ]; then
        export REGION="cn-northwest-1"
    fi

    if [ -z "${OLD_PWD}" ]; then
        export OLD_PWD="mypass"
    fi

    if [ -z "${PRESENT_PWD}" ]; then
        export PRESENT_PWD="JWTSuperSecretKeyJWTSuperSecretKeyJWTSuperSecretKeyJWTSuperSecretKey"
    fi

    
    if [ -z "${OSS_TYPE}" ]; then
        export OSS_TYPE="aws"
    fi

    export OSS_TYPE=`echo $OSS_TYPE | tr '[:upper:]' '[:lower:]'`

    if [ -z "${NS}" ]; then
        export NS="cntnp64de9wb"
    fi

    if [ "${OSS_TYPE}" == "oracle" ]; then
        export EP="$NS.compat.objectstorage.$REGION.oraclecloud.com"
        export PROVIDER="Other"
    elif [ "${OSS_TYPE}" == "aws" ]; then
        export EP="s3.$REGION.amazonaws.com.cn"
        export PROVIDER="AWS"
    elif [ "${OSS_TYPE}" == "baidu" ]; then
        export EP="s3.$REGION.bcebos.com"
        export PROVIDER="Other"
    else 
        echo "unknown oss type, only support aws, baidu or oracle"
        usage
    fi

    install_rclone
    install_kopia
    check_kopia
}

main $@