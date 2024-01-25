package main

type Signer interface {
	Sign(content []byte) (string, error)
}
