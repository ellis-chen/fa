#!/bin/bash

usage() {
    cat <<-__EOF__
Usage: $0 [-d] [-l] [-h]
-d: kill dlv server
-l: make dlv server log to dlv.log
-h: show help
__EOF__
    exit 1;
}

while getopts ":dlh" options; do
    case ${options} in
    d)
        pkill -9 dlv >/dev/null 2>&1
        exit 0
        ;;
    l)
        LOG='--log --log-dest dlv.log'
        ;;
    h)
        usage
        ;;
    :)
        echo "ERROR: Invalid option: ${OPTARG} requires an argument" 1>&2
        exit 1
        ;;
    \?)
        echo "ERROR: Invalid option: ${OPTARG}" 1>&2
        usage
        ;;
    esac
done
shift $((OPTIND - 1))

dlv --listen=:2345 --headless=true --api-version=2 --accept-multiclient $LOG attach `pgrep fa`

