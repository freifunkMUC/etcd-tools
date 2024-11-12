package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
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

func getNthSubnet(cidr string, nthSubnet uint64, subnetSize uint) (net.IP, error) {
	_, ipCidr, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR notation: %s", cidr)
	}

	baseIP := ipCidr.IP
	rangeSize, totalIPSize := ipCidr.Mask.Size()
	if subnetSize <= uint(rangeSize) {
		return nil, fmt.Errorf("subnet size %d too large for CIDR range %s", subnetSize, ipCidr.String())
	}

	subnetShiftSize := subnetSize - uint(rangeSize)
	if nthSubnet >= (1 << subnetShiftSize) {
		return nil, fmt.Errorf("cannot allocate %dx /%d subnets in the CIDR range %s", nthSubnet+1, subnetSize, ipCidr.String())
	}

	nthSubnetID := big.NewInt(int64(nthSubnet))
	nthSubnetID.Lsh(nthSubnetID, uint(totalIPSize)-subnetSize)
	nthSubnetAddr := new(big.Int).SetBytes(baseIP)
	nthSubnetAddr.Add(nthSubnetAddr, nthSubnetID)

	return net.IP(nthSubnetAddr.Bytes()), nil
}

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
			const CLIENT_V4_RANGE_SIZE uint = 22
			const CLIENT_V6_RANGE_SIZE uint = 64

			// IPv4 handling
			v4RangeStr := os.Getenv("PARKER_V4_RANGE")
			if v4RangeStr == "" {
				v4RangeStr = "10.0.0.0/8" // default IPv4 range for backwards compatibility
			}

			// IPv6 handling
			v6RangeStr := os.Getenv("PARKER_V6_RANGE")
			if v6RangeStr == "" {
				v6RangeStr = "2001:bf7:381::/48" // default IPv6 range for backwards compatibility
			}

			num := *info.ID

			v4ClientSubnet, err := getNthSubnet(v4RangeStr, num, CLIENT_V4_RANGE_SIZE)
			if err != nil {
				panic(err)
			}
			v4ClientRangeStr := fmt.Sprintf("%s/%d", v4ClientSubnet, CLIENT_V4_RANGE_SIZE)
			v4BigAddr := new(big.Int).SetBytes(v4ClientSubnet)
			v4BigAddr.Add(v4BigAddr, big.NewInt(1)) // use the next free IPv4
			v4ClientAddrStr := net.IP(v4BigAddr.Bytes()).String()

			v6ClientSubnet, err := getNthSubnet(v6RangeStr, num, CLIENT_V6_RANGE_SIZE)
			if err != nil {
				panic(err)
			}
			v6ClientRangeStr := fmt.Sprintf("%s/%d", v6ClientSubnet, CLIENT_V6_RANGE_SIZE)
			v6BigAddr := new(big.Int).SetBytes(v6ClientSubnet)
			v6BigAddr.Add(v6BigAddr, big.NewInt(1)) // use the next free IPv6
			v6ClientAddrStr := net.IP(v6BigAddr.Bytes()).String()

			info.Address4 = &v4ClientAddrStr
			info.Range4 = &v4ClientRangeStr
			info.Address6 = &v6ClientAddrStr
			info.Range6 = &v6ClientRangeStr
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
