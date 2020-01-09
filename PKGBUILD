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
_log_file="/var/log/go-check-updates.log"
_wait_time="1h"

pkgver() {
	cd "$pkgname"
  ( set -o pipefail
    git describe --long 2>/dev/null | sed 's/\([^-]*-g\)/r\1/;s/-/./g' ||
    printf "r%s.%s" "$(git rev-list --count HEAD)" "$(git rev-parse --short HEAD)"
  )   
}

build() {
    cd "${_pkgname}"
    go get -d
    go build -ldflags "-X main.defaultCache=${_cache_file} -X main.defaultWait=${_wait_time} -X main.defaultLog=${_log_file}" .
}

package() {
    cd "${_pkgname}"
    install -Dm 755 "${_pkgname}" "${pkgdir}/usr/bin/${_pkgname}"
    install -Dm 644 LICENSE "${pkgdir}/usr/share/licenses/${_pkgname}/LICENSE"
}
