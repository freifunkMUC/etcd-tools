package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/spf13/cobra"
	"paepcke.de/signify"
)

const FFBS_PUBKEY = "RWTecZzXNMuXYhvcquk321nc73U3oc7xm6Fm5FEF5y3X/HUyWsvp/rHp"

var rootCmd = &cobra.Command{
	Use:   "webutility [base url] [pubkey]",
	Short: "Utility for making web queries and verifying the response signature",
	Args:  cobra.ExactArgs(2),
	Run:   run,
}

func run(cmd *cobra.Command, args []string) {
	u, err := url.Parse(args[0])
	if err != nil {
		fmt.Println(err)
		return
	}

	pubkey, err := base64.URLEncoding.DecodeString(args[1])
	if err != nil {
		pubkey, err = base64.StdEncoding.DecodeString(args[1])
		if err != nil {
			fmt.Println("Couldn't parse pubkey as Base64 Std or URLEncoding:", err)
			return
		}
	}

	u.Path, err = url.JoinPath(u.Path, "config")
	if u.Scheme == "" {
		u.Scheme = "http"
	}
	if err != nil {
		fmt.Println(err)
		return
	}

	v := url.Values{}
	v.Set("v6mtu", "1500")
	v.Set("pubkey", base64.StdEncoding.EncodeToString(pubkey))
	v.Set("nonce", "foobar")
	u.RawQuery = v.Encode()

	resp, err := http.Get(u.String())
	if err != nil {
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return
	}

	sep := bytes.LastIndexByte(data, '\n')
	content := data[:sep+1]
	sig := data[sep+1:]

	fmt.Printf("Data: %#v\n", string(content))
	fmt.Printf("Signature: %#v\n", string(sig))

	pk := signify.NewPublicKey()
	pk.Base64 = FFBS_PUBKEY
	pk.Decode()
	msg := signify.NewMessage()
	msg.Raw = content
	msg.Signature.Base64 = string(sig)
	msg.Signature.Decode()
	msg.PublicKey = pk
	valid, err := msg.Verify(pk)
	if err != nil {
		fmt.Println(err)
		return
	}
	if valid {
		fmt.Println("The signature is valid")
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
