#!/bin/sh

showHelp() {
    cat <<-EOF
Usage: $0 [-f] [-d] [-m] [-h]

-h, -help                  Display help

-f, --format               format the given meta-url
    -f META_URL

-m, --mount                Mount the given meta-url to mount point
    -m META_URL MOUNT_POINT

-d, --destroy              Destroy the given meta-url and releated resources
    -d META_URL

EOF
}

usage() {
    showHelp
    exit 2
}

# $@ is all command line parameters passed to the script.
# -o is for short options like -v
# -l is for long options with double dash like --version
# the comma separates different long options
# -a is for long options with single dash like -version
# opts=$(getopt -l "help,format,mount,destroy" -o "hfmd" -a -- "$@")

# set --:
# If no arguments follow this option, then the positional parameters are unset. Otherwise, the positional parameters
# are set to the arguments, even if some of them begin with a ‘-’.
# eval set -- "$opts"

if [ "$*" = "--" ]; then
    usage
fi

subcommand=

while true; do
    case $1 in
    -h | --help)
        showHelp
        exit 0
        ;;
    -f | --format)
        echo "formating ..."
        subcommand=format
        shift
        break
        ;;
    -m | --mount)
        echo "mounting ..."
        subcommand=mount
        shift
        break
        ;;
    -d | --destroy)
        echo "destroying ..."
        subcommand=destroy
        shift
        break
        ;;
    --)
        subcommand=$2
        break
        ;;
    *) 
        echo "Unexpeceted option: $1 - this should not happen"
        usage
        ;;
    esac
done

# shift -- from the parameter list
# shift    

if [ "$subcommand" = "destroy" ]; then
    uuid=`juicefs status -q "$1"  | grep UUID | awk -F: '{print $2}' | tr -d '", '`
    if [ -z $uuid ]; then
        echo "not found given uuid for $1"
        exit 1
    fi
    set -- $1 $uuid --force
fi

if juicefs "$subcommand" --help >/dev/null 2>&1; then
    set -- juicefs "$subcommand" "$@"
else
    echo "= '$1' is not a juicefs command: assuming shell execution." 1>&2
fi

exec "$@"