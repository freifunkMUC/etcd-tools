package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"regexp"

	"gitli.stratum0.org/ffbs/etcd-tools/etcdhelper"
	"gitli.stratum0.org/ffbs/etcd-tools/ffbs"
	"gitli.stratum0.org/ffbs/etcd-tools/signify"

	"go.etcd.io/etcd/client/v3"
)

var SERVING_ADDR string = ":55555"

var DEFAULT_NODE_KEY string = "default"

type EtcdHandler struct {
	KV clientv3.KV
}

var ID_KEY = regexp.MustCompile(`/config/[A-Za-z0-9=_-]+/id`)

func (eh EtcdHandler) NodeCount(ctx context.Context) (uint64, error) {
	resp, err := eh.KV.Get(ctx, "/config/", clientv3.WithKeysOnly(), clientv3.WithPrefix())
	if err != nil {
		return 0, err
	}
	var count uint64
	for _, kv := range resp.Kvs {
		if ID_KEY.Match(kv.Key) {
			count++
		}
	}
	return count, nil
}

type ConcentratorInfo struct {
	Address4 string `json:"address4"`
	Address6 string `json:"address6"`
	Endpoint string `json:"endpoint"`
	PubKey   string `json:"pubkey"`
	ID       uint32 `json:"id"`
}

type NodeInfo struct {
	ID                *uint64            `json:"id,omitempty" etcd:"id"`
	Concentrators     []ConcentratorInfo `json:"concentrators,omitempty" etcd:"-"`
	ConcentratorsJSON []byte             `json:"-" etcd:"concentrators"`
	MTU               *uint64            `json:"mtu,omitempty" etcd:"mtu"`
	Retry             *uint64            `json:"retry,omitempty" etcd:"retry"`
	WGKeepalive       *uint64            `json:"wg_keepalive,omitempty" etcd:"wg_keepalive"`
	Range4            *string            `json:"range4,omitempty" etcd:"range4"`
	Range6            *string            `json:"range6,omitempty" etcd:"range6"`
	Address4          *string            `json:"address4,omitempty" etcd:"address4"`
	Address6          *string            `json:"address6,omitempty" etcd:"address6"`
}

type NodeNotFoundError struct {
	Pubkey string
}

func (err NodeNotFoundError) Error() string {
	return fmt.Sprintf("The node with the pubkey '%s' is not in etcd", err.Pubkey)
}

func (eh EtcdHandler) fillNodeInfo(pubkey string, info *NodeInfo) error {
	prefix := fmt.Sprintf("/config/%s/", pubkey)
	resp, err := eh.KV.Get(context.Background(), prefix, clientv3.WithPrefix()) // TODO remove background context
	if err != nil {
		return err
	}

	return etcdhelper.UnmarshalKVResponse(resp, info, prefix)
}

func (eh EtcdHandler) GetNodeInfo(pubkey string) (*NodeInfo, error) {
	info := &NodeInfo{}
	if err := eh.fillNodeInfo(DEFAULT_NODE_KEY, info); err != nil {
		return nil, err
	}
	if err := eh.fillNodeInfo(pubkey, info); err != nil {
		return nil, err
	}
	return info, nil
}

func main() {
	etcd, err := ffbs.CreateEtcdConnection()
	if err != nil {
		log.Fatalln("Couldn't setup etcd connection: ", err)
	}

	handler := EtcdHandler{KV: etcd.KV}

	metrics := NewMetrics(handler)

	http.Handle("/config", &ConfigHandler{tracker: metrics, signer: &signify.Cmdline{
		PrivateKey: "/etc/ffbs/node-config.sec",
	}, etcdHandler: handler})
	http.Handle("/etcd_status", metrics)

	log.Println("Starting server on", SERVING_ADDR)
	log.Fatal("Error running webserver:", http.ListenAndServe(SERVING_ADDR, nil))
}
