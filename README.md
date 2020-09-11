[![Go Report Card](https://goreportcard.com/badge/github.com/cosandr/go-check-updates)](https://goreportcard.com/report/github.com/cosandr/go-check-updates) [![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://github.com/cosandr/go-check-updates/blob/master/LICENSE)

# Introduction

A program to check for updates and make the list of updates available through a JSON file or a simple API.
By default it will check for updates every 12 hours,
if run in daemon mode then it will refresh every 12 hours,
otherwise it simply does nothing when run before 12 hours has passed since the previous update.

The default cache file may change, the first choice is `/tmp/go-check-updates.json`.
If the file exists but it isn't writable, it will fallback to `$HOME/.cache/go-check-updates/cache.json` instead.

It can be disabled completely with `-no-cache`.

The refresh interval can be changed with the `-cache.interval` option, disable with `no-refresh`.
Disabled without daemon mode will refresh every time it is run, with daemon mode there is no auto-refresh.

See `go-check-updates -h` for up to date information on the parameters.

This can then be read by other scripts, for example my own [go-motd](https://github.com/cosandr/go-motd).

## Supported package managers

Manager | Pkg | Old Ver | New Ver | Repo
--- | --- | --- | --- | ----
pacman | Y | Y | Y | N*
pikaur | Y | Y | Y | N*
dnf/yum | Y | N | Y | Y

\* Repo is simply set to "aur" or "pacman"

NOTE: dnf/yum only work with Fedora, [redhat.go](./redhat.go) is supposed to work with other distros using dnf/yum (RHEL, CentOS) however I don't know what their ID is in `/etc/os-release`. If you know, feel free to add it to the switch case in [internal.go](./internal.go)

## Installation

### Arch Linux

```sh
git clone https://github.com/cosandr/go-check-updates.git
cd go-check-updates/PKGBUILD
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
{"data":{"checked":"2020-09-11T14:47:21+02:00","updates":[{"pkg":"snapper","oldVer":"0.8.12-1","newVer":"0.8.13-1","repo":"pacman"}]},"queued":true}
# Can run directly as well (-every can be passed as argument)
$ go-check-updates
```

## Example output

Note this is what the API returns in the `data` key, the websocket returns exactly this data directly.
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
data (same as the JSON file) when updates are refreshed.

Example usage in my [Polybar setup](https://github.com/cosandr/dotfiles/blob/master/dot_config/polybar/scripts/executable_go-check-updates-ws.py).
