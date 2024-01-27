package ffbs

import (
	"context"
	"errors"
	"regexp"
	"strconv"

	"gitli.stratum0.org/ffbs/etcd-tools/etcdhelper"

	"go.etcd.io/etcd/client/v3"
)

type EtcdHandler struct {
	KV clientv3.KV
}

func (eh EtcdHandler) fillNodeInfo(ctx context.Context, pubkey string, info *NodeInfo) error {
	prefix := CONFIG_PREFIX + pubkey + "/"
	resp, err := eh.KV.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return err
	}

	if len(resp.Kvs) == 0 {
		return &NodeNotFoundError{
			Pubkey: pubkey,
		}
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

var ErrMissingNextFreeID = errors.New("Couldn't find the key for next free id")

func (eh EtcdHandler) CreateNode(ctx context.Context, pubkey string, updateNodeInfo func(*NodeInfo)) error {
	prefix := CONFIG_PREFIX + pubkey + "/"
	for {
		resp, err := eh.KV.Get(ctx, NEXT_FREE_ID_KEY)
		if err != nil {
			return err
		}
		if len(resp.Kvs) == 0 {
			return ErrMissingNextFreeID
		}
		id, err := strconv.ParseUint(string(resp.Kvs[0].Value), 10, 64)
		if err != nil {
			return err
		}

		checkID := clientv3.Compare(clientv3.Value(NEXT_FREE_ID_KEY), "=", strconv.FormatUint(id, 10))
		updateID := clientv3.OpPut(NEXT_FREE_ID_KEY, strconv.FormatUint(id+1, 10))

		nodeinfo := NodeInfo{
			ID: &id,
		}
		updateNodeInfo(&nodeinfo)

		ops := etcdhelper.Marshal(&nodeinfo, prefix)

		ops = append(ops, updateID)
		txresp, err := eh.KV.Txn(ctx).If(checkID).Then(ops...).Commit()
		if err != nil {
			return err
		}
		if txresp.Succeeded {
			return nil
		}
	}
}

var ID_KEY = regexp.MustCompile(regexp.QuoteMeta(CONFIG_PREFIX) + `([A-Za-z0-9=_-]+)/id`)

func (eh EtcdHandler) NodeCount(ctx context.Context) (uint64, error) {
	resp, err := eh.KV.Get(ctx, CONFIG_PREFIX, clientv3.WithKeysOnly(), clientv3.WithPrefix())
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

func (eh EtcdHandler) GetAllNodeInfo(ctx context.Context) (map[string]*NodeInfo, *NodeInfo, error) {
	resp, err := eh.KV.Get(ctx, CONFIG_PREFIX, clientv3.WithKeysOnly(), clientv3.WithPrefix())
	if err != nil {
		return nil, nil, err
	}

	list := make(map[string]*NodeInfo)
	// TODO Replace this inefficient loop/approach with an improved etcdhelper.UnmarshalKVResponse
	for _, kv := range resp.Kvs {
		if match := ID_KEY.FindSubmatch(kv.Key); match != nil {
			pubkey := string(match[1])
			list[pubkey], err = eh.GetOnlyNodeInfo(ctx, pubkey)
			if err != nil {
				return nil, nil, err
			}
		}
	}
	// will also be elimintated/mapped with the unmarshal, the loop doesn't catch it, as the default doesn't have an id field
	list[DEFAULT_NODE_KEY], err = eh.GetDefaultNodeInfo(ctx)
	if err != nil {
		return nil, nil, err
	}

	defaultNode := list[DEFAULT_NODE_KEY]
	delete(list, DEFAULT_NODE_KEY)
	return list, defaultNode, nil
}
