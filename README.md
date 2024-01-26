# etcd Tools

## Installing

```sh
go install gitli.stratum0.org/ffbs/etcd-tools/...@latest
```

This installs all tools to the path of the `GOBIN` environment variable (by default `~/go/bin`). By default Go uses a proxy for faster downloads and retaining the sources even when the Git-Repository goes down. The default proxy is https://proxy.golang.org and can be changed via the `GOPROXY` environment variable (the value `direct` disables the usage of a proxy).
