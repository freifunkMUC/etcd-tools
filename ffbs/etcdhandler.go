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

func (eh EtcdHandler) fillNodeInfo(ctx context.Context, pubkey string, info *NodeInfo) error {
	prefix := fmt.Sprintf("/config/%s/", pubkey)
	resp, err := eh.KV.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return err
	}

	return etcdhelper.UnmarshalKVResponse(resp, info, prefix)
}

func (eh EtcdHandler) GetOnlyNodeInfo(ctx context.Context, pubkey string) (*NodeInfo, error) {
	info := &NodeInfo{}
	err := eh.fillNodeInfo(ctx, pubkey, info)
	return info, err
}

func (eh EtcdHandler) GetDefaultNodeInfo(ctx context.Context) (*NodeInfo, error) {
	return eh.GetOnlyNodeInfo(ctx, DEFAULT_NODE_KEY)
}

func (eh EtcdHandler) GetNodeInfo(ctx context.Context, pubkey string) (*NodeInfo, error) {
	info, err := eh.GetDefaultNodeInfo(ctx)
	if err != nil {
		return nil, err
	}
	if err := eh.fillNodeInfo(ctx, pubkey, info); err != nil {
		return nil, err
	}
	return info, nil
}

var ID_KEY = regexp.MustCompile(`/config/([A-Za-z0-9=_-]+)/id`)

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

func (eh EtcdHandler) PubkeyIterator(ctx context.Context) (<-chan string, error) {
	resp, err := eh.KV.Get(ctx, "/config/", clientv3.WithKeysOnly(), clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	ch := make(chan string)
	go func() {
		defer close(ch)
		for _, kv := range resp.Kvs {
			if match := ID_KEY.FindSubmatch(kv.Key); match != nil {
				ch <- string(match[1])
			}
		}
	}()
	return ch, nil
}
