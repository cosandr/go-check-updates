# Maintainer: Andrei Costescu <andrei@costescu.no>

# shellcheck shell=bash

pkgname=go-check-updates-git
_pkgname="${pkgname%-git}"
pkgver=9879e6b
pkgrel=1
pkgdesc="Dump pending updates in a yaml file"
arch=("any")
url="https://github.com/cosandr/go-check-updates"
license=("MIT")
provides=("${_pkgname}")
conflicts=("${_pkgname}")
depends=("pacman-contrib")
optdepends=('pikaur: AUR support')
makedepends=("git" "go")
source=("git+$url")
md5sums=("SKIP")

# Change cache file and wait time durations
_cache_file="/tmp/${_pkgname}.yaml"
_log_file="/var/log/${_pkgname}.log"
_wait_time="1h"

pkgver() {
    cd "${_pkgname}"
  ( set -o pipefail
    git describe --long 2>/dev/null | sed 's/\([^-]*-g\)/r\1/;s/-/./g' ||
    printf "r%s.%s" "$(git rev-list --count HEAD)" "$(git rev-parse --short HEAD)"
  )   
}

prepare() {
    cd "${_pkgname}"

    cat > ${_pkgname}.service <<EOF
[Unit]
Description=Run ${_pkgname}

[Service]
Type=oneshot
ExecStart=/usr/bin/${_pkgname} -cache ${_cache_file} -every 1s

[Install]
WantedBy=multi-user.target
EOF

    cat > ${_pkgname}.timer <<EOF
[Unit]
Description=Run ${_pkgname}

[Timer]
# Every day at 06:00
OnCalendar=*-*-* 06:00:00
Persistent=true

[Install]
WantedBy=timers.target
EOF

    cat > 99-${_pkgname}.hook <<EOF
[Trigger]
Operation = Install
Operation = Upgrade
Type = Package
Target = *

[Action]
Description = Updating update cache
When = PostTransaction
Exec = /usr/bin/${_pkgname}
Depends = ${_pkgname}
EOF
}

build() {
    cd "${_pkgname}"
    go get -d
    go build -ldflags "-X main.defaultCache=${_cache_file} -X main.defaultWait=${_wait_time} -X main.defaultLog=${_log_file}" .
}

package() {
    cd "${_pkgname}"
    install -d "${pkgdir}/usr/lib/systemd/system"
    install -d "${pkgdir}/usr/share/libalpm/hooks"
    install -Dm 755 "${_pkgname}" "${pkgdir}/usr/bin/${_pkgname}"
    install -m 644 "${_pkgname}.service" "${pkgdir}/usr/lib/systemd/system/"
    install -m 644 "${_pkgname}.timer" "${pkgdir}/usr/lib/systemd/system/"
    install -m 644 "99-${_pkgname}.hook" "${pkgdir}/usr/share/libalpm/hooks/"
    install -Dm 644 LICENSE "${pkgdir}/usr/share/licenses/${_pkgname}/LICENSE"
}
