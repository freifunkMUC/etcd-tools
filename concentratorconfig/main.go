package main

import (
	"context"
	"log"
	"net"
	"time"

	"gitli.stratum0.org/ffbs/etcd-tools/ffbs"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const WG_DEVICENAME = "wg-nodes"

func update(etcd *ffbs.EtcdHandler, wg *wgctrl.Client) error {
	nodes, _, err := etcd.GetAllNodeInfo(context.Background())
	if err != nil {
		return err
	}

	dev, err := wg.Device(WG_DEVICENAME)
	if err != nil {
		return err
	}

	var updates wgtypes.Config
	updates.Peers = make([]wgtypes.PeerConfig, 0, 10)

	// remove and update existing nodes
	for _, peer := range dev.Peers {
		pubkey := peer.PublicKey.String()
		node, ok := nodes[pubkey]
		delete(nodes, pubkey)
		if !ok {
			// remove key as node vanished
			updates.Peers = append(updates.Peers, wgtypes.PeerConfig{
				PublicKey: peer.PublicKey,
				Remove:    true,
			})
			continue
		}

		// TODO catch range4/range6 being nil/not set
		_, range4, err := net.ParseCIDR(*node.Range4)
		if err != nil {
			return err
		}
		_, range6, err := net.ParseCIDR(*node.Range6)
		if err != nil {
			return err
		}
		// TODO check for changes and skip, if nothing changed

		hardcodedKeepalive := 15 * time.Second
		updates.Peers = append(updates.Peers, wgtypes.PeerConfig{
			PublicKey:                   peer.PublicKey,
			PersistentKeepaliveInterval: &hardcodedKeepalive,
			ReplaceAllowedIPs:           true,
			AllowedIPs:                  []net.IPNet{*range4, *range6},
		})
	}

	// add new nodes
	for pubkey, node := range nodes {
		pkey, err := wgtypes.ParseKey(pubkey)
		if err != nil {
			return err
		}

		// TODO catch range4/range6 being nil/not set
		_, range4, err := net.ParseCIDR(*node.Range4)
		if err != nil {
			return err
		}
		_, range6, err := net.ParseCIDR(*node.Range6)
		if err != nil {
			return err
		}

		hardcodedKeepalive := 15 * time.Second
		updates.Peers = append(updates.Peers, wgtypes.PeerConfig{
			PublicKey:                   pkey,
			PersistentKeepaliveInterval: &hardcodedKeepalive,
			ReplaceAllowedIPs:           true,
			AllowedIPs:                  []net.IPNet{*range4, *range6},
		})
	}

	return wg.ConfigureDevice(WG_DEVICENAME, updates)
}

func main() {
	etcd, err := ffbs.CreateEtcdConnection()
	if err != nil {
		log.Fatalln("Couldn't setup etcd connection:", err)
	}

	wg, err := wgctrl.New()
	if err != nil {
		log.Fatalln("Couldn't open connection to configure wireguard:", err)
	}

	for {
		if err := update(etcd, wg); err != nil {
			log.Println("Error trying to update the nodes:", err)
		}
		time.Sleep(60 * time.Second)
	}
}
