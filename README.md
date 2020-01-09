[![Go Report Card](https://goreportcard.com/badge/github.com/cosandr/go-check-updates)](https://goreportcard.com/report/github.com/cosandr/go-check-updates) [![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://github.com/cosandr/go-check-updates/blob/master/LICENSE)

# Introduction

This writes a yaml file, by default `/tmp/go-check-updates.yaml`, according to the global variable `defaultCache` in [updates.go](./updates.go). This can then be read by other scripts, for example my own [go-motd](https://github.com/cosandr/go-motd).

## Supported package managers

Manager | Pkg | Old Ver | New Ver | Repo
--- | --- | --- | --- | ----
pacman | Y | Y | Y | N*
pikaur | Y | Y | Y | N*
dnf/yum | Y | N | Y | Y

\* Repo is simply set to "aur" or "pacman"

NOTE: [redhat.go](./redhat/redhat.go) is supposed to work with other distros using dnf/yum (RHEL, CentOS) however I don't know what their ID is in `/etc/os-release`. If you know, feel free to add it to the switch case in [updates.go](./updates.go)

## Installation

### Arch Linux

```sh
wget https://raw.githubusercontent.com/cosandr/go-check-updates/master/PKGBUILD
makepkg -si
```

Enable `go-check-updates.timer` to run daily at 06:00 (6am) or just rely on the pacman hook which triggers after every install/update.

### Generic

Clone the repo and build a binary. Take a look at the [PKGBUILD](./PKGBUILD), particularly the default location overrides and systemd unit files in `prepare()`. Cron would work as well, note that you should provide a short `every` parameter to guarantee it will update when run.

## Example output

Arch:

```yaml
checked: 2020-01-02T18:01:45.062189725+01:00
updates:
- pkg: libarchive
  oldver: 3.4.0-3
  newver: 3.4.1-1
  repo: pacman
- pkg: libjpeg-turbo
  oldver: 2.0.3-1
  newver: 2.0.4-1
  repo: pacman
- pkg: linux
  oldver: 5.4.6.arch3-1
  newver: 5.4.7.arch1-1
  repo: pacman
- pkg: linux-headers
  oldver: 5.4.6.arch3-1
  newver: 5.4.7.arch1-1
  repo: pacman
- pkg: shellcheck
  oldver: 0.7.0-82
  newver: 0.7.0-83
  repo: pacman
```

Fedora:

```yaml
checked: 2020-01-08T06:00:05.316357064+01:00
updates:
- pkg: dnf.noarch
  newver: 4.2.17-1.fc30
  repo: updates
- pkg: dnf-data.noarch
  newver: 4.2.17-1.fc30
  repo: updates
- pkg: dnf-plugins-core.noarch
  newver: 4.0.12-1.fc30
  repo: updates
- pkg: libcomps.x86_64
  newver: 0.1.14-1.fc30
  repo: updates
- pkg: python3-dnf.noarch
  newver: 4.2.17-1.fc30
  repo: updates
- pkg: xvidcore.x86_64
  newver: 1.3.7-1.fc30
  repo: rpmfusion-free-updates
```

## Known Issues

- Is `/tmp/` a good place?
