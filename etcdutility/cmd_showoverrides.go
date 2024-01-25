package main

import (
	"context"
	"fmt"
	"log"
	"reflect"

	"gitli.stratum0.org/ffbs/etcd-tools/ffbs"

	"github.com/spf13/cobra"
)

func init() {
	cmd := &cobra.Command{
		Use:   "showoverrides",
		Short: "Shows all Pubkeys overriding a default value",
		Run:   showoverrides,
	}

	rootCmd.AddCommand(cmd)
}

func showoverrides(cmd *cobra.Command, args []string) {
	etcd, err := ffbs.CreateEtcdConnection()
	if err != nil {
		log.Fatalln("Couldn't setup etcd connection:", err)
	}

	def, err := etcd.GetDefaultNodeInfo(context.Background())
	if err != nil {
		log.Fatalln("Couldn't get default node info:", err)
	}
	defval := reflect.ValueOf(def).Elem()

	unchanged := make(map[string]uint64)
	for _, field := range reflect.VisibleFields(defval.Type()) {
		unchanged[field.Name] = 0
	}

	it, err := etcd.PubkeyIterator(context.Background())
	if err != nil {
		log.Fatalln("Couldn't iterate pubkeys:", err)
	}
	for pubkey := range it {
		nodeinfo, err := etcd.GetOnlyNodeInfo(context.Background(), pubkey)
		if err != nil {
			log.Fatalln("Couldn't get nodeinfo for", pubkey, err)
		}
		nodeinfovalue := reflect.ValueOf(nodeinfo).Elem()

		for _, field := range reflect.VisibleFields(defval.Type()) {
			d := defval.FieldByIndex(field.Index)
			v := nodeinfovalue.FieldByIndex(field.Index)

			if d.IsNil() {
				continue
			}
			if v.IsNil() {
				unchanged[field.Name]++
				continue
			}

			if v.Kind() == reflect.Pointer {
				v = v.Elem()
				d = d.Elem()
			}
			if fmt.Sprintf("%s", v.Interface()) != fmt.Sprintf("%s", d.Interface()) {
				fmt.Println("Overridden", field.Name, "for", pubkey, "with value", v.Interface())
			}
		}
	}

	fmt.Println("Nodes affected by the corresponding default values:", unchanged)
}
