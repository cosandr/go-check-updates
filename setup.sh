#!/bin/bash

set -e -o pipefail -o noclobber -o nounset

! getopt --test > /dev/null
if [[ ${PIPESTATUS[0]} -ne 4 ]]; then
    echo '`getopt --test` failed in this environment.'
    exit 1
fi

OPTIONS=h,v,n
LONGOPTS=help,bin-path:,build-path:,cache-file:,cache-interval:,hook-path:,listen-address:,log-file:,no-cache,no-log,no-refresh,pkg-name:,systemd-path:,sysconfig-path:,no-watch,watch-interval:,verbose,dry-run,tmp-path:

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
SYSCONFIG_PATH="/etc/sysconfig"
WATCH_INTERVAL="10s"
TMP_DIR="/tmp/build-$PKG_NAME"
BUILD_DIR="./build"
VERBOSE=0
DRY_RUN=0
_cwd=$(pwd -P)

function print_help {
# Using a here doc with standard out.
echo "Usage $0: COMMAND [OPTIONS]

Commands:
build                 Build subcommands, output $BUILD_DIR
    bin               Build for all platforms (${PLATFORMS[*]})
    pacman            Copy required files to build a pacman package from local files
    rpm               Copy required files to build an RPM package from local files
install               Install binary and systemd units
    hook              Install pacman hooks
    systemd           Install systemd socket and service files

Options:
-h    --help            Show this message
-v    --verbose         Show generated files
-n    --dry-run         Don't install files
      --bin-path        Path where the binary is installed (default $BIN_PATH)
      --build-path      Path where built stuff is copied to (default $BUILD_DIR)
      --cache-file      Change default cache file location (default $CACHE_FILE)
      --cache-interval  Change auto-refresh interval (default $CACHE_INTERVAL)
      --hook-path       Path where pacman hooks are installed (default $HOOK_PATH)
      --listen-address  Listen address (default $LISTEN_ADDRESS)
      --log-file        Change default log file location (default $LOG_FILE)
      --no-cache        Disable cache file
      --no-log          Disable log file
      --no-refresh      Disable auto-refresh
      --no-watch        Disable package manager log file watching
      --pkg-name        Change package name (default $PKG_NAME)
      --systemd-path    Path where systemd units are installed (default $SYSTEMD_PATH)
      --sysconfig-path  Path where env file is installed (default $SYSCONFIG_PATH)
      --tmp-path        Path where temporary build files are copied to (default $TMP_DIR)
      --watch-interval  Change watch interval (default $WATCH_INTERVAL)
"
}

while true; do
    case "$1" in
        -h|--help)
            print_help
            exit 0
            ;;
        -v|--verbose)
            VERBOSE=1
            shift
            ;;
        -n|--dry-run)
            DRY_RUN=1
            shift
            ;;
        --bin-path)
            BIN_PATH="$2"
            shift 2
            ;;
        --build-path)
            BUILD_DIR="$2"
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
        --no-watch)
            WATCH_INTERVAL=""
            shift
            ;;
        --pkg-name)
            PKG_NAME="$2"
            LOG_FILE="/var/log/${PKG_NAME}.log"
            CACHE_FILE="/tmp/${PKG_NAME}.json"
            LISTEN_ADDRESS="/run/$PKG_NAME.sock"
            TMP_DIR="/tmp/build-$PKG_NAME"
            shift 2
            ;;
        --sysconfig-path)
            SYSCONFIG_PATH="$2"
            shift 2
            ;;
        --systemd-path)
            SYSTEMD_PATH="$2"
            shift 2
            ;;
        --tmp-path)
            TMP_DIR="$2"
            shift 2
            ;;
        --watch-interval)
            WATCH_INTERVAL="$2"
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

if [[ $# -lt 1 ]]; then
    echo "$0: A command is required."
    exit 4
fi

PKG_PATH="$BIN_PATH/$PKG_NAME"
[[ $VERBOSE -eq 1 ]] && verbose_arg="-v" || verbose_arg=""

function generate_systemd {
    _nl=$'\n'
    systemd_env=""
    if [[ -n $CACHE_FILE ]]; then
        systemd_env+="CACHE_FILE=\"$CACHE_FILE\"${_nl}"
    else
        systemd_env+="NO_CACHE=1${_nl}"
    fi
    if [[ -n $CACHE_INTERVAL ]]; then
        systemd_env+="CACHE_INTERVAL=\"$CACHE_INTERVAL\"${_nl}"
    else
        systemd_env+="NO_REFRESH=1${_nl}"
    fi
    if [[ -n $LOG_FILE ]]; then
        systemd_env+="LOG_FILE=\"$LOG_FILE\"${_nl}"
    else
        systemd_env+="NO_LOG_FILE=1${_nl}"
    fi
    if [[ -n $WATCH_INTERVAL ]]; then
        systemd_env+="WATCH_ENABLE=1${_nl}"
        systemd_env+="WATCH_INTERVAL=\"$WATCH_INTERVAL\"${_nl}"
    else
        systemd_env+="WATCH_ENABLE=0${_nl}"
    fi
    tmp_sock="${TMP_DIR}/$PKG_NAME.socket"
    tmp_serv="${TMP_DIR}/$PKG_NAME.service"
    tmp_env="${TMP_DIR}/$PKG_NAME.env"
    env_file="${SYSCONFIG_PATH}/$PKG_NAME"
    echo "$systemd_env" > "$tmp_env"
    cat <<EOF > "$tmp_sock"
[Socket]
ListenStream=$LISTEN_ADDRESS
BindIPv6Only=both

[Install]
WantedBy=sockets.target
EOF
    cat <<EOF > "$tmp_serv"
[Unit]
Description=$PKG_NAME service
After=network.target
Requires=network.target

[Service]
EnvironmentFile=-$env_file
ExecStart=$PKG_PATH --systemd
EOF
    # Don't overwrite existing file
    [[ -f $env_file ]] && env_file+=".new"
    # Print if verbose
    if [[ $VERBOSE -eq 1 ]]; then
        echo -e "\n\t$PKG_NAME.socket"
        cat "$tmp_sock"
        echo -e "\n\t$PKG_NAME.service"
        cat "$tmp_serv"
        echo -e "\n\t$env_file"
        cat "$tmp_env"
    fi
    if [[ $DRY_RUN -ne 1 ]]; then
        install -m 0644 $verbose_arg "$tmp_sock" "${SYSTEMD_PATH}/"
        install -m 0644 $verbose_arg "$tmp_serv" "${SYSTEMD_PATH}/"
        install -m 0640 $verbose_arg "$tmp_env" "${env_file}"
    fi
}

function generate_hook {
    query="?refresh&log_file"
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
    tmp_hook="${TMP_DIR}/$PKG_NAME.hook"
    cat <<EOF > "$tmp_hook"
[Trigger]
Operation = Remove
Operation = Upgrade
Type = Package
Target = *

[Action]
Description = Refresh $PKG_NAME
When = PostTransaction
Exec = /usr/bin/curl $CURL_ADDRESS --silent --output /dev/null '$GET_URL'
Depends = curl
EOF
    # Print if verbose
    if [[ $VERBOSE -eq 1 ]]; then
        echo -e "\n\t$PKG_NAME.hook"
        cat "$tmp_hook"
    fi
    [[ $DRY_RUN -ne 1 ]] && install -m 0644 $verbose_arg "$tmp_hook" "${HOOK_PATH}/"
}

function build_binary {
    tmp_bin="${TMP_DIR}/$PKG_NAME"
    go build $verbose_arg -o "$tmp_bin"
    [[ $DRY_RUN -ne 1 ]] && install -m 0755 $verbose_arg "$tmp_bin" "${BIN_PATH}/"
}

function build_all_binaries {
    for platform in "${PLATFORMS[@]}"; do
        IFS="/" read -r -a platform_split <<< "$platform"
        GOOS=${platform_split[0]}
        GOARCH=${platform_split[1]}
        output_name=$PKG_NAME'-'$GOOS'-'$GOARCH
        echo "Building $output_name"
        export GOOS
        export GOARCH
        export CGO_ENABLED=0
        mkdir -p "$BUILD_DIR"
        if ! go build $verbose_arg -o "${BUILD_DIR}/$output_name"; then
            echo 'Build failed'
            exit 1
        fi
    done
}

function build_pacman {
    mkdir -p "$TMP_DIR"/src/"$PKG_NAME"
    rsync -a $verbose_arg ./ "$TMP_DIR"/src/"$PKG_NAME" \
        --exclude build \
        --exclude packaging \
        --filter=':- .gitignore'
    cp -f ./packaging/pacman/* "$TMP_DIR"/
    cd "$TMP_DIR"
    makepkg --noextract
    cd "$_cwd"
    mkdir -p "$BUILD_DIR"
    mv -f $verbose_arg "$TMP_DIR"/"$PKG_NAME"*.pkg.tar.* "$BUILD_DIR"/
}

function build_rpm {
    if [[ ! -d ~/rpmbuild/SOURCES ]]; then
        if ! rpmdev-setuptree; then
            echo "Install rpmdev-tools"
            exit 1
        fi
    fi
    rpm_name="golang-github-cosandr-check-updates"
    spec_file="./packaging/rpm/$rpm_name.spec"
    tag=$(git describe --tags --abbrev=0)
    # Create SPEC file if it doesn't exist already
    if [[ ! -f $spec_file ]]; then
        echo "Creating SPEC file"
        go2rpm github.com/cosandr/go-check-updates -t "$tag" --stdout > "$spec_file"
        echo "Modify SPEC file and re-run"
        exit 0
    fi
    # Prepare source
    src_name="go-check-updates-${tag#v}"
    rm -rf $verbose_arg "${TMP_DIR:?}"/"$src_name"
    mkdir -p "$TMP_DIR"/"$src_name"
    rsync -a $verbose_arg ./ "$TMP_DIR"/"$src_name" \
        --exclude vendor \
        --exclude build \
        --exclude packaging \
        --filter=':- .gitignore'
    cd "$TMP_DIR"
    tar -czf $verbose_arg ~/rpmbuild/SOURCES/"$src_name".tar.gz "$src_name"
    cd "$_cwd"
    rpmbuild -bb --nocheck "$spec_file"
    mkdir -p "$BUILD_DIR"
    rm -f $verbose_arg "$BUILD_DIR"/*.rpm
    mv -f $verbose_arg ~/rpmbuild/RPMS/x86_64/"$PKG_NAME-0-"*.rpm "$BUILD_DIR"/
    # Cleanup
    echo "CLEANUP"
    [[ $VERBOSE -eq 1 ]] && _args="-print" || _args=""
    set +e
    find ~/rpmbuild/ -name "$PKG_NAME*" $_args -exec rm -rf {} \;
    find ~/rpmbuild/ -name "$rpm_name*" $_args -exec rm -rf {} \;
}

# Cleanup before doing anything
rm -rf "$TMP_DIR" 2>/dev/null
mkdir -p "$TMP_DIR"

function cleanup {
    [[ -d "$TMP_DIR" ]] && rm -rf "$TMP_DIR"
}

trap cleanup EXIT

case "$1" in
    build)
        if [[ $# -ne 2 ]]; then
            echo "$0 $1: A subcommand is required."
            exit 4
        fi
        case "$2" in
            bin)
                build_all_binaries
                ;;
            pacman)
                build_pacman
                ;;
            rpm)
                build_rpm
                ;;
            *)
                echo "Unrecognized $1 subcommand: $2"
                print_help
                exit 2
                ;;
        esac
        ;;
    install)
        if [[ $# -ne 2 ]]; then
            build_binary
            generate_systemd
            exit 0
        fi
        case "$2" in
            hook)
                generate_hook
                ;;
            systemd)
                generate_systemd
                ;;
            *)
                echo "Unrecognized $1 subcommand: $2"
                print_help
                exit 2
                ;;
        esac
        ;;
    *)
        echo "Unrecognized command: $1"
        print_help
        exit 2
        ;;
esac
