package ffbs

import (
	"context"
	"errors"
	"regexp"
	"strconv"

	"gitli.stratum0.org/ffbs/etcd-tools/etcdhelper"

	"go.etcd.io/etcd/client/v3"
)

// Implements all Freifunk Braunschweig specific etcd interactions to keep the etcd
// specific details away from the application logic.
type EtcdHandler struct {
	KV clientv3.KV
}

func (eh EtcdHandler) fillNodeInfo(ctx context.Context, pubkey string, info *NodeInfo) error {
	prefix := CONFIG_PREFIX + pubkey + "/"
	applied, err := etcdhelper.UnmarshalGet(ctx, eh.KV, prefix, info)

	if err == nil && applied == 0 {
		return &NodeNotFoundError{
			Pubkey: pubkey,
		}
	}
	return err
}

// Get only the specific node info stored at the /config/[pubkey] etcd prefix.
//
// Use this function only if you don't want the default values. Otherwise consider using [EtcdHandler.GetNodeInfo]
func (eh EtcdHandler) GetOnlyNodeInfo(ctx context.Context, pubkey string) (*NodeInfo, error) {
	info := &NodeInfo{}
	err := eh.fillNodeInfo(ctx, pubkey, info)
	return info, err
}

// Get the default node info stored at the /config/default etcd prefix.
func (eh EtcdHandler) GetDefaultNodeInfo(ctx context.Context) (*NodeInfo, error) {
	return eh.GetOnlyNodeInfo(ctx, DEFAULT_NODE_KEY)
}

// Get the node info fo a given Wireguard pubkey.
//
// This function will use the [EtcdHandler.GetDefaultNodeInfo] values as a basis and override them with
// the specific node information from [EtcdHandler.GetOnlyNodeInfo]
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

// Indicates that the [NEXT_FREE_ID_KEY] is not present in the etcd instance
var ErrMissingNextFreeID = errors.New("Couldn't find the key for next free id")

// Adds a new node to the etcd KV store.
//
// This function will retrieve a free node id and initialize a [NodeInfo] struct using it.
// Afterwards it calls the updateNodeInfo function to fill the struct and inserts the results into etcd.
// The function may be called multiple times if the node id was already claimed when inserting the node into etcd.
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

// Returns the number of node configurations stored in etcd
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

// Retrieves all [NodeInfo] stored in etcd.
//
// The second argument retuns the default node value. It is equivalent to calling [EtcdHandler.GetDefaultNodeInfo]
//
// The returned slice of nodes don't have the default values applied, see [EtcdHandler.GetOnlyNodeInfo]
func (eh EtcdHandler) GetAllNodeInfo(ctx context.Context) (map[string]*NodeInfo, *NodeInfo, error) {
	list := make(map[string]*NodeInfo)
	if _, err := etcdhelper.UnmarshalGet(ctx, eh.KV, CONFIG_PREFIX, &list); err != nil {
		return nil, nil, err
	}

	defaultNode := list[DEFAULT_NODE_KEY]
	delete(list, DEFAULT_NODE_KEY)
	return list, defaultNode, nil
}
