package main

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"gitli.stratum0.org/ffbs/etcd-tools/ffbs"
)

type ConfigResponse struct {
	ffbs.NodeInfo
	Nonce string `json:"nonce"`
	Time  int64  `json:"time"`
}

type ConfigHandler struct {
	tracker     RequestTracker
	signer      Signer
	etcdHandler *ffbs.EtcdHandler
}

var MISSING_V6MTU = errors.New("Missing v6mtu query parameter")
var MISSING_PUBKEY = errors.New("Missing pubkey query parameter")
var MISSING_NONCE = errors.New("Missing nonce query parameter")

func (ch ConfigHandler) handleRequest(ctx context.Context, query url.Values, headers http.Header) (*ConfigResponse, error) {
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
		pk, err := base64.StdEncoding.DecodeString(key)
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

	nodeinfo, err := ch.etcdHandler.GetNodeInfo(ctx, pubkey)
	if err != nil {
		var notfoundError *ffbs.NodeNotFoundError
		if !errors.As(err, &notfoundError) {
			return nil, err
		}

		// insert new node
		err := ch.etcdHandler.CreateNode(ctx, pubkey, func(info *ffbs.NodeInfo) {
			v4_base_str := os.Getenv("PARKER_V4_BASE")
			v4_range_size_str := os.Getenv("PARKER_V4_RANGE_SIZE")
			v6_base_high_str := os.Getenv("PARKER_V6_BASE_HIGH")

			if v4_base_str == "" {
				v4_base_str = "10"
			}
			if v4_range_size_str == "" {
				v4_range_size_str = "10"
			}
			if v6_base_high_str == "" {
				v6_base_high_str = "0x20010bf70381"
			}

			v4_base_uint32, err := strconv.ParseInt(v4_base_str, 0, 32)
			V4_BASE := uint32(v4_base_uint32) << 24
			if err != nil {
				panic(err)
			}

			v4_range_uint8, err := strconv.ParseInt(v4_range_size_str, 0, 8)
			V4_RANGE_SIZE := uint8(v4_range_uint8)
			if err != nil {
				panic(err)
			}

			v6_base_high_uint64, err := strconv.ParseInt(v6_base_high_str, 0, 64)
			V6_BASE_HIGH := uint64(v6_base_high_uint64) << 16
			if err != nil {
				panic(err)
			}

			num := *info.ID

			var v4Addr [net.IPv4len]byte
			binary.BigEndian.PutUint32(v4Addr[:], V4_BASE|(uint32(num)<<V4_RANGE_SIZE))
			v4range := fmt.Sprintf("%s/%d", net.IP(v4Addr[:]), 8*net.IPv4len-V4_RANGE_SIZE)
			v4Addr[net.IPv4len-1] = 1
			v4addr := net.IP(v4Addr[:]).String()

			var v6Addr [net.IPv6len]byte
			binary.BigEndian.PutUint64(v6Addr[:8], V6_BASE_HIGH|uint64(num))
			v6range := fmt.Sprintf("%s/64", net.IP(v6Addr[:]))
			v6Addr[net.IPv6len-1] = 1
			v6addr := net.IP(v6Addr[:]).String()

			info.Address4 = &v4addr
			info.Range4 = &v4range
			info.Address6 = &v6addr
			info.Range6 = &v6range
		})
		if err != nil {
			return nil, err
		}
		nodeinfo, err = ch.etcdHandler.GetNodeInfo(ctx, pubkey)
		if err != nil {
			return nil, err
		}
	}

	if err := json.Unmarshal(nodeinfo.ConcentratorsJSON, &nodeinfo.Concentrators); err != nil {
		return nil, err
	}

	concentratorBitmask := nodeinfo.SelectedConcentratorsBitmask()
	var i uint
	var resolver net.Resolver
	for _, concentrator := range nodeinfo.Concentrators {
		if (concentratorBitmask>>(concentrator.ID-1))&1 == 0 {
			continue // Concentrator not enabled in bitmask
		}

		host, port, err := net.SplitHostPort(concentrator.Endpoint)
		if err != nil {
			log.Println("Error splitting concentrator endpoint host/ip", concentrator.Endpoint, ":", err)
			continue
		}
		network := "ip"
		if forceIPv4 {
			network = "ip4"
		}
		ip, err := resolver.LookupIP(ctx, network, host)
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

	resp, err := ch.handleRequest(req.Context(), req.URL.Query(), req.Header)
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
			toSign = append(toSign, '\n')
			if signature, err := ch.signer.Sign(toSign); err != nil {
				log.Println("Error signing response:", err)
				ch.tracker.RequestFailed()
			} else {
				if _, err = w.Write(toSign); err != nil {
					log.Println("Error writing json response:", err)
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

	panicked = false
}
