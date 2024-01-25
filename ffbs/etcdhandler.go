package ffbs

import (
	"context"
	"fmt"
	"regexp"

	"gitli.stratum0.org/ffbs/etcd-tools/etcdhelper"

	"go.etcd.io/etcd/client/v3"
)

type EtcdHandler struct {
	KV clientv3.KV
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
