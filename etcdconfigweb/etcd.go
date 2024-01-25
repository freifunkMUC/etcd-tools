package main

import (
	"os"
	"encoding/json"
	"strings"
	"crypto/x509"
	"crypto/tls"
	"io"

	"go.etcd.io/etcd/client/v3"
)

type EtcdConfigFile struct {
	Endpoints string // comma separated
	CACert string
	Cert string
	Key string
}

func CreateEtcdConnection() (*clientv3.Client, error) {
	f, err := os.Open("/etc/etcd-client.json")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cfg EtcdConfigFile
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, err
	}

	f, err = os.Open(cfg.CACert)
	if err != nil {
		return nil, err
	}
	cacontents, err := io.ReadAll(f)
	f.Close()
	if err != nil {
		return nil, err
	}
	rootCAs := x509.NewCertPool()
	rootCAs.AppendCertsFromPEM(cacontents)

	cert, err := tls.LoadX509KeyPair(cfg.Cert, cfg.Key)
	if err != nil {
		return nil, err
	}

	return clientv3.New(clientv3.Config{
		Endpoints: strings.Split(cfg.Endpoints, ","),
		TLS: &tls.Config{
			Certificates: []tls.Certificate{ cert },
			RootCAs: rootCAs,
		},
	})
}
