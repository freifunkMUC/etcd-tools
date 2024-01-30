package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"os"
	"sort"
	"time"

	"gitli.stratum0.org/ffbs/etcd-tools/ffbs"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const WG_DEVICENAME = "wg-nodes"

func sortIPNet(s []net.IPNet) {
	sort.Slice(s, func(i, j int) bool {
		res := bytes.Compare(s[i].IP[:], s[j].IP[:])
		if res != 0 {
			return res > 0
		}
		res = bytes.Compare(s[i].Mask[:], s[j].Mask[:])
		return res > 0
	})
}

func calculateWGPeerUpdates(etcd *ffbs.EtcdHandler, wg *wgctrl.Client) ([]wgtypes.PeerConfig, error) {
	nodes, defNode, err := etcd.GetAllNodeInfo(context.Background())
	if err != nil {
		return nil, err
	}

	dev, err := wg.Device(WG_DEVICENAME)
	if err != nil {
		return nil, err
	}

	updates := make([]wgtypes.PeerConfig, 0, 10)

	// remove and update existing nodes
	for _, peer := range dev.Peers {
		pubkey := base64.URLEncoding.EncodeToString(peer.PublicKey[:])
		node, ok := nodes[pubkey]
		delete(nodes, pubkey)
		if !ok {
			// remove key as node vanished
			updates = append(updates, wgtypes.PeerConfig{
				PublicKey: peer.PublicKey,
				Remove:    true,
			})
			continue
		}

		nets := node.IPNets()
		sortIPNet(nets)
		sortIPNet(peer.AllowedIPs)

		equalNet := true
		if len(nets) != len(peer.AllowedIPs) {
			equalNet = false
		} else {
			for i, cur := range nets {
				if !bytes.Equal(cur.IP[:], peer.AllowedIPs[i].IP[:]) {
					equalNet = false
					break
				}
				if !bytes.Equal(cur.Mask[:], peer.AllowedIPs[i].Mask[:]) {
					equalNet = false
					break
				}
			}
		}

		keepalive := node.WGKeepaliveTime()
		if keepalive == nil {
			keepalive = defNode.WGKeepaliveTime()
			if keepalive == nil {
				disable := 0 * time.Second
				keepalive = &disable
			}
		}
		keepaliveChanged := *keepalive != peer.PersistentKeepaliveInterval

		if !equalNet || keepaliveChanged {
			updates = append(updates, wgtypes.PeerConfig{
				PublicKey:                   peer.PublicKey,
				PersistentKeepaliveInterval: keepalive,
				ReplaceAllowedIPs:           true,
				AllowedIPs:                  nets,
			})
		}
	}

	// add new nodes
	for pubkey, node := range nodes {
		decpkey, err := base64.URLEncoding.DecodeString(pubkey)
		if err != nil {
			return nil, fmt.Errorf("couldn't base64 decode pubkey '%s'", pubkey)
		}
		pkey, err := wgtypes.NewKey(decpkey)
		if err != nil {
			return nil, err
		}

		keepalive := node.WGKeepaliveTime()
		if keepalive == nil {
			keepalive = defNode.WGKeepaliveTime()
			if keepalive == nil {
				disable := 0 * time.Second
				keepalive = &disable
			}
		}

		updates = append(updates, wgtypes.PeerConfig{
			PublicKey:                   pkey,
			PersistentKeepaliveInterval: keepalive,
			ReplaceAllowedIPs:           true,
			AllowedIPs:                  node.IPNets(),
		})
	}

	return updates, nil
}

func main() {
	simulate := false
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "simulate":
			simulate = true
		default:
			log.Fatalln("unknown arguments. Currently only 'simulate' is supported")
		}
	}

	etcd, err := ffbs.CreateEtcdConnection()
	if err != nil {
		log.Fatalln("Couldn't setup etcd connection:", err)
	}

	wg, err := wgctrl.New()
	if err != nil {
		log.Fatalln("Couldn't open connection to configure wireguard:", err)
	}

	for {
		// misusing a loop to break at any moment and still run the sleep call
		for {
			updates, err := calculateWGPeerUpdates(etcd, wg)
			if err != nil {
				log.Println("Error trying to determine the node updates:", err)
				break
			}
			if simulate {
				fmt.Printf("Peer updates: %v\n", updates)
				return
			}
			if len(updates) == 0 {
				break
			}

			if err := wg.ConfigureDevice(WG_DEVICENAME, wgtypes.Config{Peers: updates}); err != nil {
				log.Println("Error trying to apply the node updates:", err)
				break
			}
			log.Println("Updated", len(updates), "peers")
			break
		}
		time.Sleep(60 * time.Second)
	}
}
