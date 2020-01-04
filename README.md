
# What does this do?

Writes a yaml file, by default `/tmp/go-check-updates.yaml`, according to the global variable `defaultCache` in [main.go](./main.go). This can then be read by other scripts, for example my own [go-motd](https://github.com/cosandr/go-motd).

## Supported package managers

Manager | Pkg | Old Ver | New Ver | Repo
--- | --- | --- | --- | ----
pacman | Y | Y | Y | N*
pikaur | Y | Y | Y | N*
dnf/yum | Y | N | Y | Y

\* Repo is simply set to "aur" or "pacman"

NOTE: [redhat.go](./redhat/redhat.go) is supposed to work with other distros using dnf/yum (RHEL, CentOS) however I don't know what their ID is in `/etc/os-release`. If you know, feel free to add it to the switch case in [main.go](./main.go)

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
checked: 2020-01-02T16:38:19.884383209+01:00
updates:
- pkg: pgdg-fedora-repo.noarch
  newver: 42.0-6
  repo: pgdg12
- pkg: pgdg-fedora-repo.noarch
  newver: 42.0-6
  repo: pgdg11
- pkg: pgdg-fedora-repo.noarch
  newver: 42.0-6
  repo: pgdg10
- pkg: pgdg-fedora-repo.noarch
  newver: 42.0-6
  repo: pgdg96
- pkg: pgdg-fedora-repo.noarch
  newver: 42.0-6
  repo: pgdg95
- pkg: pgdg-fedora-repo.noarch
  newver: 42.0-6
  repo: pgdg94
```

## Known Issues

- Is `/tmp/` a good place?
