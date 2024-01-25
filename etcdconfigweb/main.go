package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gitli.stratum0.org/ffbs/etcd-tools/etcdhelper"
	"gitli.stratum0.org/ffbs/etcd-tools/signify"

	"go.etcd.io/etcd/client/v3"
)

const RETRY_TIME_FOR_REGISTERED uint = 600

var SERVING_ADDR string = ":55555"

type ConfigResponse struct {
	NodeInfo
	Nonce string `json:"nonce"`
	Time  int64  `json:"time"`
}

var DEFAULT_NODE_KEY string = "default"

type EtcdHandler struct {
	KV clientv3.KV
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

type NodeNotFoundError struct {
	Pubkey string
}

func (err NodeNotFoundError) Error() string {
	return fmt.Sprintf("The node with the pubkey '%s' is not in etcd", err.Pubkey)
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

type Signer interface {
	Sign(content []byte) (string, error)
}

type ConfigHandler struct {
	tracker     RequestTracker
	signer      Signer
	etcdHandler EtcdHandler
}

var MISSING_V6MTU = errors.New("Missing v6mtu query parameter")
var MISSING_PUBKEY = errors.New("Missing pubkey query parameter")
var MISSING_NONCE = errors.New("Missing nonce query parameter")

func (ch ConfigHandler) handleRequest(query url.Values, headers http.Header) (*ConfigResponse, error) {
	var v6mtu uint64
	var err error
	if mtu := query["v6mtu"]; len(mtu) > 0 {
		v6mtu, err = strconv.ParseUint(mtu[0], 10, 16)
		if err != nil {
			return nil, fmt.Errorf("Couldn't convert v6mtu '%s' to an integer: %w", mtu, err)
		}
	} else {
		return nil, MISSING_V6MTU
	}

	var pubkey string
	if key := query.Get("pubkey"); key != "" {
		pk, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(key, " ", "+"))
		if err != nil {
			return nil, fmt.Errorf("Couldn't decode the provided pubkey '%s': %w", key, err)
		}
		if len(pk) != 32 {
			return nil, fmt.Errorf("Expected the pubkey to have 32 bytes, but it has %d bytes instead", len(pk))
		}
		pubkey = base64.URLEncoding.EncodeToString(pk)
	} else {
		return nil, MISSING_PUBKEY
	}

	nonce := query.Get("nonce")
	if nonce == "" {
		return nil, MISSING_NONCE
	}

	forceIPv4 := !strings.Contains(headers.Get("X-Real-IP"), ":")
	if v6mtu < 1455 {
		// 1375+40+8+4+4+8+16, see https://www.mail-archive.com/wireguard@lists.zx2c4.com/msg01856.html
		forceIPv4 = true
		log.Println("v6mtu", v6mtu, "too small, using v4")
	}

	nodeinfo, err := ch.etcdHandler.GetNodeInfo(pubkey)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(nodeinfo.ConcentratorsJSON, &nodeinfo.Concentrators); err != nil {
		return nil, err
	}

	var i uint
	var resolver net.Resolver
	for _, concentrator := range nodeinfo.Concentrators {
		host, port, err := net.SplitHostPort(concentrator.Endpoint)
		if err != nil {
			log.Println("Error splitting concentrator endpoint host/ip", concentrator.Endpoint, ":", err)
			continue
		}
		network := "ip"
		if forceIPv4 {
			network = "ip4"
		}
		ip, err := resolver.LookupIP(context.Background(), network, host) // TODO better context
		if err != nil {
			// Fail the whole response, as smth. seems to be broken on our resolve side
			return nil, err
		}
		if len(ip) == 0 {
			continue
		}
		concentrator.Endpoint = net.JoinHostPort(ip[0].String(), port)
		nodeinfo.Concentrators[i] = concentrator
		i++
	}
	nodeinfo.Concentrators = nodeinfo.Concentrators[:i]

	return &ConfigResponse{
		Nonce:    nonce,
		Time:     time.Now().Unix(),
		NodeInfo: *nodeinfo,
	}, nil
}

func (ch ConfigHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	panicked := true
	defer func() {
		if panicked {
			ch.tracker.RequestFailed()
		}
	}()

	resp, err := ch.handleRequest(req.URL.Query(), req.Header)
	if err != nil {
		fmt.Println("Error while handling configuration request:", err)
		w.WriteHeader(http.StatusBadRequest)
		ch.tracker.RequestFailed()
	} else {
		w.Header().Add("Content-Type", "text/plain")

		if toSign, err := json.Marshal(resp); err != nil {
			log.Println("Couldn't encode JSON response:", err)
			ch.tracker.RequestFailed()
		} else {
			if signature, err := ch.signer.Sign(toSign); err != nil {
				log.Println("Error signing response:", err)
				ch.tracker.RequestFailed()
			} else {
				if _, err = w.Write(toSign); err != nil {
					log.Println("Error writing json response:", err)
					ch.tracker.RequestFailed()
				} else {
					if _, err = w.Write([]byte{'\n'}); err != nil {
						log.Println("Error writing newline:", err)
						ch.tracker.RequestFailed()
					} else {

						if _, err = w.Write([]byte(signature)); err != nil {
							log.Println("Error writing signature: ", err)
							ch.tracker.RequestFailed()
						} else {
							ch.tracker.RequestSuccessful()
						}
					}
				}
			}
		}
	}

	panicked = false
}

func main() {
	etcd, err := CreateEtcdConnection()
	if err != nil {
		log.Fatalln("Couldn't setup etcd connection: ", err)
	}

	handler := EtcdHandler{KV: etcd.KV}

	metrics := NewMetrics(handler)

	http.Handle("/config", &ConfigHandler{tracker: metrics, signer: &signify.Cmdline{
		PrivateKey: "/etc/ffbs/node-config.sec",
	}, etcdHandler: handler})
	http.Handle("/etcd_status", metrics)

	log.Println("Starting server on", SERVING_ADDR)
	log.Fatal("Error running webserver:", http.ListenAndServe(SERVING_ADDR, nil))
}
