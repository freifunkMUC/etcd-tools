package ffbs

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
