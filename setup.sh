#!/bin/bash

set -e -o pipefail -o noclobber -o nounset

! getopt --test > /dev/null
if [[ ${PIPESTATUS[0]} -ne 4 ]]; then
    echo '`getopt --test` failed in this environment.'
    exit 1
fi

OPTIONS=h
LONGOPTS=help,bin-path:,cache-file:,cache-interval:,hook-path:,listen-address:,log-file:,no-cache,no-log,no-refresh,pkg-name:,systemd-path:

! PARSED=$(getopt --options=$OPTIONS --longoptions=$LONGOPTS --name "$0" -- "$@")
if [[ ${PIPESTATUS[0]} -ne 0 ]]; then
    exit 2
fi

eval set -- "$PARSED"

### DEFAULTS ###

PLATFORMS=("linux/386" "linux/amd64")
PKG_NAME="go-check-updates"
BIN_PATH="/usr/bin"
CACHE_FILE="/tmp/${PKG_NAME}.json"
CACHE_INTERVAL="12h"
HOOK_PATH="/usr/share/libalpm/hooks"
LISTEN_ADDRESS="/run/$PKG_NAME.sock"
LOG_FILE="/var/log/${PKG_NAME}.log"
SYSTEMD_PATH="/usr/lib/systemd/system"

print_help () {
# Using a here doc with standard out.
echo "Usage $0: COMMAND [OPTIONS]

Commands:
build-all             Build for all platforms (${PLATFORMS[*]})
install               Build and install binary
pacman-build          Copy required files to build a pacman package from local files
pacman-hook           Create and install pacman hooks
systemd               Create and install systemd socket and service files

Options:
-h    --help            Show this message
      --bin-path        Path where the binary is installed (default $BIN_PATH)
      --cache-file      Change default cache file location (default $CACHE_FILE)
      --cache-interval  Change auto-refresh interval (default $CACHE_INTERVAL)
      --hook-path       Path where pacman hooks are installed (default $HOOK_PATH)
      --listen-address  Listen address (default $LISTEN_ADDRESS)
      --log-file        Change default log file location (default $LOG_FILE)
      --no-cache        Disable cache file
      --no-log          Disable log file
      --no-refresh      Disable auto-refresh
      --pkg-name        Change package name (default $PKG_NAME)
      --systemd-path    Path where systemd units are installed (default $SYSTEMD_PATH)
"
}

while true; do
    case "$1" in
        -h|--help)
            print_help
            exit 0
            ;;
        --bin-path)
            BIN_PATH="$2"
            shift 2
            ;;
        --cache-file)
            CACHE_FILE="$2"
            shift 2
            ;;
        --cache-interval)
            CACHE_INTERVAL="$2"
            shift 2
            ;;
        --hook-path)
            HOOK_PATH="$2"
            shift 2
            ;;
        --listen-address)
            LISTEN_ADDRESS="$2"
            shift 2
            ;;
        --log-file)
            LOG_FILE="$2"
            shift 2
            ;;
        --no-cache)
            CACHE_FILE=""
            shift
            ;;
        --no-log)
            LOG_FILE=""
            shift
            ;;
        --no-refresh)
            CACHE_INTERVAL=""
            shift
            ;;
        --pkg-name)
            PKG_NAME="$2"
            LOG_FILE="/var/log/${PKG_NAME}.log"
            CACHE_FILE="/tmp/${PKG_NAME}.json"
            LISTEN_ADDRESS="/run/$PKG_NAME.sock"
            shift 2
            ;;
        --systemd-path)
            SYSTEMD_PATH="$2"
            shift 2
            ;;
        --)
            shift
            break
            ;;
        *)
            echo "Programming error"
            exit 3
            ;;
    esac
done

if [[ $# -ne 1 ]]; then
    echo "$0: A command is required."
    exit 4
fi

PKG_PATH="$BIN_PATH/$PKG_NAME"
SOCKET_FILE="$SYSTEMD_PATH/$PKG_NAME.socket"
SERVICE_FILE="$SYSTEMD_PATH/$PKG_NAME.service"
HOOK_FILE="$HOOK_PATH/$PKG_NAME.hook"

case "$1" in
    build-all)
        for platform in "${PLATFORMS[@]}"
        do
            IFS="/" read -r -a platform_split <<< "$platform"
            GOOS=${platform_split[0]}
            GOARCH=${platform_split[1]}
            output_name=$PKG_NAME'-'$GOOS'-'$GOARCH
            echo "Building $output_name"
            if ! env CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build -o $output_name; then
                echo 'Build failed'
                exit 1
            fi
        done
        ;;
    install)
        go build -o "$PKG_PATH"
        ;;
    systemd)
        _nl=$'\n'
        systemd_env=""
        if [[ -n $CACHE_FILE ]]; then
            systemd_env+="Environment=CACHE_FILE=\"$CACHE_FILE\"${_nl}"
        else
            systemd_env+="Environment=NO_CACHE=1${_nl}"
        fi
        if [[ -n $CACHE_INTERVAL ]]; then
            systemd_env+="Environment=CACHE_INTERVAL=\"$CACHE_INTERVAL\"${_nl}"
        else
            systemd_env+="Environment=NO_REFRESH=1${_nl}"
        fi
        if [[ -n $LOG_FILE ]]; then
            systemd_env+="Environment=LOG_FILE=\"$LOG_FILE\"${_nl}"
        else
            systemd_env+="Environment=NO_LOG=1${_nl}"
        fi
        set +e
        echo -e "\n########## Systemd socket ##########\n"
        cat <<EOF | tee "$SOCKET_FILE"
[Socket]
ListenStream=$LISTEN_ADDRESS
BindIPv6Only=both

[Install]
WantedBy=sockets.target
EOF
        echo -e "\n########## Systemd service ##########\n"
        cat <<EOF | tee "$SERVICE_FILE"
[Unit]
Description=$PKG_NAME service
After=network.target
Requires=network.target

[Service]
$systemd_env
ExecStart=$PKG_PATH -systemd
EOF
        ;;
    pacman-hook)
        query="?refresh&immediate&every=5m"
        # We have a unix socket
        if [[ $LISTEN_ADDRESS == /* ]]; then
            CURL_ADDRESS="--unix-socket $LISTEN_ADDRESS"
            GET_URL="http://localhost/api$query"
        # Listen anywhere address, add localhost
        elif [[ $LISTEN_ADDRESS == :* || $LISTEN_ADDRESS == 0.0.0.0:* ]]; then
            CURL_ADDRESS=""
            IFS=":" read -r -a address_split <<< "$LISTEN_ADDRESS"
            GET_URL="http://localhost:${address_split[1]}/api$query"
        # Use as is
        else
            CURL_ADDRESS=""
            GET_URL="http://${LISTEN_ADDRESS}/api$query"
        fi
        echo -e "\n########## Pacman hook ##########\n"
        cat <<EOF | tee "$HOOK_FILE"
[Trigger]
Operation = Remove
Operation = Upgrade
Type = Package
Target = *

[Action]
Description = Queue cache update for $PKG_NAME
When = PostTransaction
Exec = /usr/bin/curl $CURL_ADDRESS --silent --output /dev/null '$GET_URL'
Depends = curl
EOF
        ;;
    pacman-build)
        rm -rf ./build
        mkdir -p ./build/src/"$PKG_NAME"
        rsync -a ./ ./build/src/"$PKG_NAME" --exclude build --exclude PKGBUILD
        cp -f ./PKGBUILD/* ./build/
        cd ./build
        makepkg --noextract
        ;;
    *)
        echo "Unrecognized command: $1"
        print_help
        exit 2
        ;;
esac
