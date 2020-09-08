[![Go Report Card](https://goreportcard.com/badge/github.com/cosandr/go-check-updates)](https://goreportcard.com/report/github.com/cosandr/go-check-updates) [![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://github.com/cosandr/go-check-updates/blob/master/LICENSE)

# Introduction

This writes a json file, by default `/tmp/go-check-updates.json`, according to the global variable `defaultCache` in [updates.go](./updates.go). This can then be read by other scripts, for example my own [go-motd](https://github.com/cosandr/go-motd).

## Supported package managers

Manager | Pkg | Old Ver | New Ver | Repo
--- | --- | --- | --- | ----
pacman | Y | Y | Y | N*
pikaur | Y | Y | Y | N*
dnf/yum | Y | N | Y | Y

\* Repo is simply set to "aur" or "pacman"

NOTE: [redhat.go](./redhat.go) is supposed to work with other distros using dnf/yum (RHEL, CentOS) however I don't know what their ID is in `/etc/os-release`. If you know, feel free to add it to the switch case in [updates.go](./updates.go)

## Installation

### Arch Linux

```sh
wget https://raw.githubusercontent.com/cosandr/go-check-updates/master/PKGBUILD
makepkg -si
```

Enable and start `go-check-updates.socket`, the pacman hook triggers after every update/remove.

### Generic

Use [setup.sh](setup.sh), read the help it prints out `setup.sh -h`

## Usage

Assuming it is listening on `localhost:8100`.
See API section for more details.

```sh
# Update now, returns after update has completed
$ curl 'http://localhost:8100/api?refresh'
{}
# Update now, return file location immediately
$ curl 'http://localhost:8100/api?refresh&filepath&immediate'
{"filePath":"/tmp/go-check-updates.json","queued":true}
# Get current list of updates
$ curl 'http://localhost:8100/api?updates'
{"data":{"checked":"2020-06-02T13:28:16+02:00","updates":[]}}
# Get current updates, update if file is older than 1 hour and return immediately
# Status code will be 202 and the "queued" key will be present and true if an update was queued
# If no update is needed, status code is 200 and there is no queued key present
$ curl 'http://localhost:8100/api?refresh&updates&immediate&every=1h'
# Can run directly as well (-every can be passed as argument)
$ go-check-updates
```

## Example output

```json
{
  "checked": "2020-06-01T23:10:23+02:00",
  "updates": [
    {
      "pkg": "archiso",
      "oldVer": "43-2",
      "newVer": "44-2",
      "repo": "pacman"
    },
    {
      "pkg": "ca-certificates-mozilla",
      "oldVer": "3.52.1-2",
      "newVer": "3.53-1",
      "repo": "pacman"
    },
    {
      "pkg": "imagemagick",
      "oldVer": "7.0.10.15-1",
      "newVer": "7.0.10.16-2",
      "repo": "pacman"
    }
  ]
}
```


## API

Run with `-daemon` argument to start a web server,
listen address and port can be adjusted with `-web.listen-address`.

Alternatively, systemd socket activation can be used with the `-systemd` argument, socket and service units can be
created with the `setup.sh` script.

`/api` endpoint

One of these parameters must be present:

- `filepath` returns path to the cache file in use
- `updates` returns currently cached updates
- `refresh` refreshes cached update list, the other commands run after this one. The following parameters can
be combined with this one
  - `every` value parsed as time duration, it will only refresh if the file is older than this duration
  - `immediate` won't wait for the request to finish before returning, returned data (if requested) is likely
    out of date
    
Status codes:

- `200` request was successful
- `400` bad argument(s)
- `202` update queued
- `500` something went wrong server side, `Error` is included in response with more details

## Websocket

Requires web server (daemon or systemd mode). Connect to `/ws` endpoint to receive
data (same as the JSON file) when the cache file is updated.

## Known Issues

- Is `/tmp/` a good place?
