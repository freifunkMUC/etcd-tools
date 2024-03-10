// Freifunk Braunschweig specific structures and helper functionality
package ffbs

import (
	"net"
	"strconv"
	"strings"
	"time"
)

// Concentrator configuration encoded as a JSON string in the /config/[pubkey]/concentrator key
type ConcentratorInfo struct {
	Address4 string `json:"address4"`
	Address6 string `json:"address6"`
	Endpoint string `json:"endpoint"`
	PubKey   string `json:"pubkey"`
	ID       uint32 `json:"id"`
}

// The node specific configuration values stored in the /config/[pubkey] etcd prefix.
//
// A special node info lives in the /config/default etcd prefix, which is usually used
// to fill all missing values from the individual nodes.
type NodeInfo struct {
	ID                    *uint64            `json:"id,omitempty" etcd:"id"`
	Concentrators         []ConcentratorInfo `json:"concentrators,omitempty" etcd:"-"`
	ConcentratorsJSON     []byte             `json:"-" etcd:"concentrators"`
	MTU                   *uint64            `json:"mtu,omitempty" etcd:"mtu"`
	Retry                 *uint64            `json:"retry,omitempty" etcd:"retry"`
	WGKeepalive           *uint64            `json:"wg_keepalive,omitempty" etcd:"wg_keepalive"`
	Range4                *string            `json:"range4,omitempty" etcd:"range4"`
	Range6                *string            `json:"range6,omitempty" etcd:"range6"`
	Address4              *string            `json:"address4,omitempty" etcd:"address4"`
	Address6              *string            `json:"address6,omitempty" etcd:"address6"`
	SelectedConcentrators *string            `json:"-" etcd:"selected_concentrators"`
}

// Returns a bitmask starting from the least significant bit indicating the concentrators to
// use. Due to the return type this only allows for a maximum of 64 concentrators.
// In case of no defined concentrator, all concentrators will be selected
func (ni NodeInfo) SelectedConcentratorsBitmask() uint64 {
	if ni.SelectedConcentrators == nil {
		return ^uint64(0)
	}

	var ret uint64
	for _, num := range strings.Split(*ni.SelectedConcentrators, " ") {
		n, err := strconv.ParseUint(num, 10, 64)
		if err != nil {
			continue // ignore...
		}
		if n > 64 || n == 0 {
			panic("Concentrator number 0 or > 64 found. This is not supported")
		}
		ret |= 1 << (n - 1)
	}
	if ret == 0 {
		return ^uint64(0)
	}
	return ret
}

// Returns the parsed Range4/Range6 values. Empty and invalid values are omitted.
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

// Parses the KeepaliveTime into an [time.Duration]
//
// If the keepalive time is nil, it also returns nil
func (ni NodeInfo) WGKeepaliveTime() *time.Duration {
	if ni.WGKeepalive == nil {
		return nil
	}

	res := time.Duration(*ni.WGKeepalive) * time.Second
	return &res
}
