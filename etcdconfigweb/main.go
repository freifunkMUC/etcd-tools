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
