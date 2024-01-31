/*
etcdconfigweb provides an http interface to query and register nodes from the etcd KV store.

By default it will listen on port 8080 on any interface. You can change this by passing an argument
like ":1234", which would configure to listen on port 1234 on any interface or "127.0.0.1:1234" to only listen
on the IPv4 local address "127.0.0.1" on port "1234".

It expects an etcd configuration file at a fixed location (see [gitli.stratum0.org/ffbs/etcd-tools/ffbs.CreateEtcdConnection])
and a signify private key to sign the requests at "/etc/ffbs/node-config.sec"

As it doesn't need any root capabilities, it should be considered to run this executable as a normal user.

The HTTP server supports two endpoints:
  - /config to retrieve node configurations or create new nodes
  - /etcd_status to retrieve the current node count in etcd and the amount of successful and failed requests to the /config endpoint
*/
package main

import (
	"log"
	"net/http"
	"os"

	"gitli.stratum0.org/ffbs/etcd-tools/ffbs"
)

func main() {
	etcd, err := ffbs.CreateEtcdConnection()
	if err != nil {
		log.Fatalln("Couldn't setup etcd connection: ", err)
	}

	servingAddr := ":8080"
	if len(os.Args) > 1 {
		servingAddr = os.Args[1]
	}

	signer, err := NewSignifySignerFromPrivateKeyFile("/etc/ffbs/node-config.sec")
	if err != nil {
		log.Fatalln("Couldn't parse signify private key:", err)
	}

	metrics := NewMetrics(etcd)

	http.Handle("/config", &ConfigHandler{tracker: metrics, signer: signer, etcdHandler: etcd})
	http.Handle("/etcd_status", metrics)

	log.Println("Starting server on", servingAddr)
	log.Fatal("Error running webserver:", http.ListenAndServe(servingAddr, nil))
}
