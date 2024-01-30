package ffbs

import (
	"net"
	"time"
)

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

func (ni NodeInfo) IPNets() []net.IPNet {
	nets := make([]net.IPNet, 0, 2)
	_, range4, err := net.ParseCIDR(*ni.Range4)
	if err == nil && range4 != nil {
		nets = append(nets, *range4)
	}
	_, range6, err := net.ParseCIDR(*ni.Range6)
	if err == nil && range6 != nil {
		nets = append(nets, *range6)
	}

	return nets
}

func (ni NodeInfo) WGKeepaliveTime() *time.Duration {
	if ni.WGKeepalive == nil {
		return nil
	}

	res := time.Duration(*ni.WGKeepalive) * time.Second
	return &res
}
