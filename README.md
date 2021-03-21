# sko
`sko` (simple ko) allows to build and pusblish Go applications directly without the need for Docker or a Dockerfile.
`sko` is a stripped-down version of [ko](https://github.com/google/ko). `sko` leaves out all the Kubernetes integrations and only tries to offer more intuitive interface for the `ko publish` command.

## Install
```
go install github.com/dvob/sko@latest
```

## Usage
```
Build and upload to local docker daemon:
```
sko -local dvob/http-server .
```

Build and push to Docker Hub:
```
sko dvob/http-server .
```

Build and push certain tags:
```
sko -tag latest -tag v0.0.7 dvob/http-server .
```

Login:
```
sko -login
```
