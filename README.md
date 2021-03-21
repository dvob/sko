# sko
`sko` (simple ko) allows to build and pusblish Go applications directly without the need for Docker or a Dockerfile.
`sko` is a stripped-down version of [ko](https://github.com/google/ko). `sko` leaves out all the Kubernetes integrations and tries to offer more intuitive interface for the `ko publish` command.

## Install
Use `go install`:
```shell
go install github.com/dvob/sko@latest
```
Or download from [releases](https://github.com/dvob/sko/releases):
```shell
curl -L -sS https://github.com/dvob/sko/releases/download/v0.0.1/sko_0.0.1_linux_amd64.tar.gz | tar -C ~/bin -xzf - sko
```

## Usage
Build and upload to local docker daemon:
```shell
sko -local dvob/http-server .
sko -local dvob/foo ./cmd/foo
```

Build and push to a registry:
```shell
# docker hub
sko dvob/http-server .

# full qualified
sko quay.io/foo/bar .
```

Build and push certain tags:
```shell
sko -tag latest -tag v0.0.7 dvob/http-server .
```

Set username for push to registry.
```shell
sko -user dvob -password sUp3r53cret dvob/bla .

# or from environment
export SKO_USER=dvob
export SKO_PASSWORD=sUp3r53cret
sko dvob/blabla .
```
If no user and password are set `sko` uses credentials from Docker (e.g. `~/.docker/config.json`):


### Github Actions
For an example on how to use `sko` in a Github Actions workflow check out [http-server](https://github.com/dvob/http-server/blob/81d9adf0808b1f6e27353a5d027e20ac031bd70c/.github/workflows/main.yml#L33)
