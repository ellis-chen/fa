#!/bin/sh

# This script will select the latest release version of JuiceFS based on your operating system and install it in /usr/local/bin.

# usage:
#       sh install-juicefs.sh
set -e

fatal(){
    echo '[ERROR] ' "$@" >&2
    exit 1
}

# shellcheck disable=SC2021
# shellcheck disable=SC2005
OPERATING_SYSTEM=$(echo "$(uname -s)" | tr '[A-Z]' '[a-z]')
CPU_ARCHITECTURE=""
DOWNLOADER=""
JFS_LATEST_TAG=""
TMP_DIR=""
FILE_NAME=""

# --- set arch , fatal if architecture not supported ---
setup_verify_arch() {
    case $(uname -m) in
        amd64)
            CPU_ARCHITECTURE="amd64"
            ;;
        x86_64)
            CPU_ARCHITECTURE="amd64"
            ;;
        arm64)
            CPU_ARCHITECTURE="arm64"
            ;;
        aarch64)
            CPU_ARCHITECTURE="arm64"
            ;;
        *)
            fatal "Unsupported architecture $(uname -m)"
    esac
}

# --- verify existence of network downloader executable ---
verify_downloader() {
    # Return failure if it doesn't exist or is no executable
    [ -x "$(command -v "$1")" ] || return 1

    # Set verified executable as our downloader program and return success
    DOWNLOADER=$1
    return 0
}

# --- create temporary directory and cleanup when done ---
setup_tmp() {
    TMP_DIR=$(mktemp -d)
    cleanup() {
        code=$?
        set +e
        trap - EXIT
        rm -rf "${TMP_DIR}"
        exit $code
    }
    trap cleanup INT EXIT
}

progress_filter (){
    local flag=false c count cr=$'\r' nl=$'\n'
    while IFS='' read -d '' -rn 1 c
    do
        if $flag
        then
            printf '%s' "$c"
        else
            if [[ $c != $cr && $c != $nl ]]
            then
                count=0
            else
                ((count++))
                if ((count > 1))
                then
                    flag=true
                fi
            fi
        fi
    done
}

download() {
    [ $# -eq 2 ] || fatal 'download needs exactly 2 arguments'
    echo "Downloading ${FILE_NAME}"
    case $DOWNLOADER in
        curl)
            curl -o "$2" -fL "$1"
            ;;
        wget)
            set +e
            wget --help | grep  -q  '\--show-progress'
            # shellcheck disable=SC2181
            if [ $? -eq 0 ]; then
            # –-show-progress option is only available since GNU wget 1.16
                wget -O "$2" "$1" -q --show-progress
            else
                wget -O "$2" "$1" --progress=bar:force 2>&1 | progress_filter
            fi
            set -e
            ;;
        *)
            fatal "Incorrect executable '$DOWNLOADER'"
            ;;
    esac

    # Abort if download command failed
    # shellcheck disable=SC2181
    [ $? -eq 0 ] || fatal 'Download failed'
}

setup_jfs_latest_tag(){
    case $DOWNLOADER in
        curl)
            JFS_LATEST_TAG=$(curl -s https://api.github.com/repos/juicedata/juicefs/releases/latest | grep 'tag_name' | cut -d '"' -f 4 | tr -d 'v')
            ;;
        wget)
            JFS_LATEST_TAG=$(wget -qO- https://api.github.com/repos/juicedata/juicefs/releases/latest | grep 'tag_name' | cut -d '"' -f 4 | tr -d 'v')
            ;;
        *)
            fatal "Incorrect downloader executable '$DOWNLOADER'"
            ;;
    esac
    if [ -z "$JFS_LATEST_TAG" ]; then
        fatal "Failed to get latest juicefs tag"
    fi
    echo "Latest JuiceFS version is: $JFS_LATEST_TAG"
}

setup_downloader(){
  verify_downloader wget || verify_downloader curl || fatal 'Can not find curl or wget for downloading files'
}


{
  setup_verify_arch
  setup_downloader
  setup_jfs_latest_tag
  setup_tmp
  FILE_NAME="juicefs-${JFS_LATEST_TAG}-${OPERATING_SYSTEM}-${CPU_ARCHITECTURE}.tar.gz"
  download "https://d.juicefs.com/juicefs/releases/download/v${JFS_LATEST_TAG}/${FILE_NAME}" "${TMP_DIR}/${FILE_NAME}"
  tar -zxf "${TMP_DIR}/${FILE_NAME}" -C  "${TMP_DIR}"
  if [ -O "/usr/local/bin" ]
  then
    mv -f "${TMP_DIR}/juicefs" "/usr/local/bin/juicefs"
  else
    echo "Install juicefs to /usr/local/bin requires root permission"
  	sudo mv -f "${TMP_DIR}/juicefs" "/usr/local/bin/juicefs"
  fi
  echo "Install juicefs to /usr/local/bin/juicefs successfully!"
}
