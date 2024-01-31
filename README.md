# etcd Tools

A set of tools used by Freifunk Braunschweig to interact with an [etcd](https://etcd.io) Key Value store.

This includes:
- etcdconfigweb ([Godoc](https://pkg.go.dev/gitli.stratum0.org/ffbs/etcd-tools/etcdconfigweb)) to provide a HTTP server for querying node configurations and inserting new nodes into etcd.
- concentratorconfig ([Godoc](https://pkg.go.dev/gitli.stratum0.org/ffbs/etcd-tools/concentratorconfig)) retrieves the list of nodes from the etcd and updates the wireguard interface accordingly.
- etcdutility ([Godoc](https://pkg.go.dev/gitli.stratum0.org/ffbs/etcd-tools/etcdutility)) is a command line utility with specialized functions for the FFBS etcd instance.
- webutility ([Godoc](https://pkg.go.dev/gitli.stratum0.org/ffbs/etcd-tools/webutility)) is an basic client to interact with etcdconfigweb for debugging purposes.

## Installing

```sh
go install gitli.stratum0.org/ffbs/etcd-tools/...@latest
```

This installs all tools to the path of the `GOBIN` environment variable (by default `~/go/bin`). By default Go uses a proxy for faster downloads and retaining the sources even when the Git-Repository goes down. The default proxy is https://proxy.golang.org and can be changed via the `GOPROXY` environment variable (the value `direct` disables the usage of a proxy).
