package ffbs

import (
	"fmt"
)

type NodeNotFoundError struct {
	Pubkey string
}

func (err NodeNotFoundError) Error() string {
	return fmt.Sprintf("The node with the pubkey '%s' is not in etcd", err.Pubkey)
}
