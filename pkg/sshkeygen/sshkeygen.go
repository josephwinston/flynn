package sshkeygen

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"

	"github.com/flynn/flynn/Godeps/_workspace/src/golang.org/x/crypto/ssh"
)

type SSHKey struct {
	PublicKey  []byte
	PrivateKey *rsa.PrivateKey
}

// This generates a single RSA 2048-bit SSH key
func Generate() (*SSHKey, error) {
	data := &SSHKey{}

	var pemBuf bytes.Buffer

	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	pem.Encode(&pemBuf, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(rsaKey),
	})
	rsaPubKey, err := ssh.NewPublicKey(&rsaKey.PublicKey)
	if err != nil {
		return nil, err
	}
	data.PublicKey = bytes.TrimSpace(ssh.MarshalAuthorizedKey(rsaPubKey))
	data.PrivateKey = rsaKey

	return data, nil
}
