/*
Utility for the ffbs etcd. Currently it can only show all nodes overriding a default value and
the number of nodes affected when chaning the default value.

See the help page (pass "--help" as argument) for further documentation.
*/
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "etcdutility",
	Short: "Utility for some ffbs etcd management tasks",
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
