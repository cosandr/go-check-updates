#!/bin/bash

set -e -o pipefail -o noclobber -o nounset

! getopt --test > /dev/null
if [[ ${PIPESTATUS[0]} -ne 4 ]]; then
    echo '`getopt --test` failed in this environment.'
    exit 1
fi

OPTIONS=hno:
LONGOPTS=help,dry-run,output:,listen-address:,pkg-name:,cache-file:,log-file:,bin-path:,systemd-path:,hook-path:

! PARSED=$(getopt --options=$OPTIONS --longoptions=$LONGOPTS --name "$0" -- "$@")
if [[ ${PIPESTATUS[0]} -ne 0 ]]; then
    exit 2
fi

eval set -- "$PARSED"

### DEFAULTS ###

PKG_NAME="go-check-updates"
BIN_PATH="/usr/bin"
SYSTEMD_PATH="/usr/lib/systemd/system"
HOOK_PATH="/usr/share/libalpm/hooks"
LOG_FILE="/var/log/${PKG_NAME}.log"
CACHE_FILE="/tmp/${PKG_NAME}.json"
LISTEN_ADDRESS="/run/$PKG_NAME.sock"
LOG_FILE_CHANGED=0
CACHE_FILE_CHANGED=0
LISTEN_ADDRESS_CHANGED=0

function print_help () {
# Using a here doc with standard out.
cat <<-END
Usage $0: COMMAND [OPTIONS]

Commands:
install               Build and install binary
systemd-unit          Create and install systemd socket and service files
systemd-timer         Create and install systemd timer and service files
pacman-hook           Create and install pacman hooks
pacman-build          Copy required files to build a pacman package from local files

Options:
-h    --help            Show this message
      --listen-address  Listen address (default $LISTEN_ADDRESS)
      --pkg-name        Change package name (default $PKG_NAME)
      --cache-file      Change default cache file location (default $CACHE_FILE)
      --log-file        Change default log file location (default $LOG_FILE)
      --bin-path        Path where the binary is installed (default $BIN_PATH)
      --systemd-path    Path where systemd units are installed (default $SYSTEMD_PATH)
      --hook-path       Path where pacman hooks are installed (default $HOOK_PATH)
END
}

while true; do
    case "$1" in
        -h|--help)
            print_help
            exit 0
            ;;
        --pkg-name)
            PKG_NAME="$2"
            shift 2
            ;;
        --listen-address)
            LISTEN_ADDRESS="$2"
            LISTEN_ADDRESS_CHANGED=1
            shift 2
            ;;
        --log-file)
            LOG_FILE="$2"
            LOG_FILE_CHANGED=1
            shift 2
            ;;
        --cache-file)
            CACHE_FILE="$2"
            CACHE_FILE_CHANGED=1
            shift 2
            ;;
        --bin-path)
            BIN_PATH="$2"
            shift 2
            ;;
        --systemd-path)
            SYSTEMD_PATH="$2"
            shift 2
            ;;
        --hook-path)
            HOOK_PATH="$2"
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

if [[ $LOG_FILE_CHANGED -eq 0 ]]; then
    LOG_FILE="/var/log/${PKG_NAME}.log"
fi

if [[ $CACHE_FILE_CHANGED -eq 0 ]]; then
    CACHE_FILE="/tmp/${PKG_NAME}.json"
LISTEN_ADDRESS="/run/$PKG_NAME.sock"
fi

if [[ $LISTEN_ADDRESS_CHANGED -eq 0 ]]; then
    LISTEN_ADDRESS="/run/$PKG_NAME.sock"
fi

PKG_PATH="$BIN_PATH/$PKG_NAME"
SOCKET_FILE="$SYSTEMD_PATH/$PKG_NAME.socket"
SERVICE_FILE="$SYSTEMD_PATH/$PKG_NAME.service"
TIMER_FILE="$SYSTEMD_PATH/$PKG_NAME-timer.timer"
TIMER_SERVICE="$SYSTEMD_PATH/$PKG_NAME-timer.service"
HOOK_FILE="$HOOK_PATH/$PKG_NAME.hook"


# We have a unix socket
if [[ $LISTEN_ADDRESS == /* ]]; then
    CURL_ADDRESS="--unix-socket $LISTEN_ADDRESS"
# Listen anywhere address, add localhost
elif [[ $LISTEN_ADDRESS == :* ]]; then
    CURL_ADDRESS="localhost${LISTEN_ADDRESS}"
# Use as is
else
    CURL_ADDRESS="$LISTEN_ADDRESS"
fi

case "$1" in
    install)
        go build -o "$PKG_PATH" -ldflags "-X main.defaultCache=${CACHE_FILE} -X main.defaultLog=${LOG_FILE}"
        ;;
    systemd-unit)
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
ExecStart=$PKG_PATH -systemd
EOF
        ;;
    systemd-timer)
        echo -e "\n########## Systemd timer ##########\n"
        cat <<EOF | tee "$TIMER_FILE"
[Unit]
Description=Run $PKG_NAME

[Timer]
# Every hour
OnBootSec=10s
OnUnitActiveSec=1h
Persistent=true

[Install]
WantedBy=timers.target
EOF
        echo -e "\n########## Systemd timer service ##########\n"
        cat <<EOF | tee "$TIMER_SERVICE"
[Unit]
Description=Run $PKG_NAME
Requires=$SOCKET_FILE

[Service]
Type=oneshot
ExecStart=/usr/bin/curl $CURL_ADDRESS --silent --output /dev/null 'http://localhost/api?refresh&immediate'

[Install]
WantedBy=multi-user.target
EOF
        ;;
    pacman-hook)
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
Exec = /usr/bin/curl $CURL_ADDRESS --silent --output /dev/null 'http://localhost/api?refresh&immediate'
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
