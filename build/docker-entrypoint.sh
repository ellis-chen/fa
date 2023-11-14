#!/bin/sh
set -Eeo pipefail

err() {
    echo "Error exited with status $? at line $1 with trace:"
    awk 'NR>L-4 && NR<L+4 { printf "%-5d%3s%s\n",NR,(NR==L?">>>":""),$0 }' L=$1 $0
}
trap 'err $LINENO' ERR

if [ -n "$TIKV_META_HOST" ] && [ $(command -v juicefs 2>&1 >/dev/null; echo $?) = 0 ] ; then
    juicefs mount -d tikv://$TIKV_META_HOST/$FB_LOC_PREFIX $FA_STORE_PATH --cache-size ${JUICE_CACHE_SIZE:-128} --metrics 0.0.0.0:9567
fi

# first arg is `-f` or `--some-option`
if [ "${1#*-}" != "$1" ] && [ "$1" != "fa" ]; then
    set -- fa "$@"
fi

# if our command is a valid fa subcommand, let's invoke it through fa instead
# (this allows for "docker run fa version", etc)
if fa "$1" --help >/dev/null 2>&1; then
    set -- fa "$@"
else
    echo "= '$1' is not a fa command: assuming shell execution." 1>&2
fi

exec "$@"
