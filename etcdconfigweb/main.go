package main

import (
	"log"
	"net/http"
	"os"

	"gitli.stratum0.org/ffbs/etcd-tools/ffbs"
	"gitli.stratum0.org/ffbs/etcd-tools/signify"
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

	metrics := NewMetrics(etcd)

	http.Handle("/config", &ConfigHandler{tracker: metrics, signer: &signify.Cmdline{
		PrivateKey: "/etc/ffbs/node-config.sec",
	}, etcdHandler: etcd})
	http.Handle("/etcd_status", metrics)

	log.Println("Starting server on", servingAddr)
	log.Fatal("Error running webserver:", http.ListenAndServe(servingAddr, nil))
}
