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
	"go.seankhliao.com/signify"
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
	signature := string(data[sep+1:])

	fmt.Printf("Data: %#v\n", string(content))
	fmt.Printf("Signature: %#v\n", signature)

	pkbin, err := base64.StdEncoding.DecodeString(FFBS_PUBKEY)
	if err != nil {
		fmt.Println(err)
		return
	}
	pk, err := signify.ParsePublicKey(pkbin)
	if err != nil {
		fmt.Println(err)
		return
	}
	sigbin, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		fmt.Println(err)
		return
	}
	sig, err := signify.ParseSignature(sigbin)
	if err != nil {
		fmt.Println(err)
		return
	}

	if signify.Verify(pk, content, sig) {
		fmt.Println("The signature is valid")
	} else {
		fmt.Println("The signature is INVALID!!!")
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
