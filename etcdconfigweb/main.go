package main

import (
	"log"
	"net/http"

	"gitli.stratum0.org/ffbs/etcd-tools/ffbs"
	"gitli.stratum0.org/ffbs/etcd-tools/signify"
)

var SERVING_ADDR string = ":55555"

func main() {
	etcd, err := ffbs.CreateEtcdConnection()
	if err != nil {
		log.Fatalln("Couldn't setup etcd connection: ", err)
	}

	metrics := NewMetrics(etcd)

	http.Handle("/config", &ConfigHandler{tracker: metrics, signer: &signify.Cmdline{
		PrivateKey: "/etc/ffbs/node-config.sec",
	}, etcdHandler: etcd})
	http.Handle("/etcd_status", metrics)

	log.Println("Starting server on", SERVING_ADDR)
	log.Fatal("Error running webserver:", http.ListenAndServe(SERVING_ADDR, nil))
}
