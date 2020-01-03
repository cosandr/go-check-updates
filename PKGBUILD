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
_wait_time="1h"

pkgver() {
    cd "${srcdir}/${_pkgname}"
    git describe --always
}

build() {
    cd "${_pkgname}"
    go get -d
    go build -ldflags "-X main.defaultCache=${_cache_file} -X main.defaultWait=${_wait_time}" .
}

package() {
    cd "${_pkgname}"
    install -Dm 755 "${srcdir}/${_pkgname}/${_pkgname}" "${pkgdir}/usr/bin/${_pkgname}"
    install -Dm 644 LICENSE "${pkgdir}/usr/share/licenses/${_pkgname}/LICENSE"
}
