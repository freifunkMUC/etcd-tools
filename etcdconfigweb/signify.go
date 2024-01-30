package main

import (
	"bufio"
	"encoding/base64"
	"os"

	"go.seankhliao.com/signify"
)

type SignifySigner signify.PrivateKey

func (ss *SignifySigner) Sign(data []byte) (string, error) {
	sig := signify.Sign((*signify.PrivateKey)(ss), data)
	msig := signify.MarshalSignature(sig)
	return base64.StdEncoding.EncodeToString(msig), nil
}

func NewSignifySignerFromPrivateKeyFile(file string) (*SignifySigner, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	br := bufio.NewReader(f)
	var key []byte
	for {
		line, _, err := br.ReadLine()
		if err != nil {
			return nil, nil
		}
		if key, err = base64.StdEncoding.DecodeString(string(line)); err == nil {
			break
		}
	}

	pk, err := signify.ParsePrivateKey(key, nil)
	return (*SignifySigner)(pk), err
}
