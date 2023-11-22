package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"github.com/quic-go/quic-go"
	"log"
	"math/big"
)

const addr = "localhost:4242"

func main() {
	conns := make(map[quic.Connection]struct{})

	go func() {
		err := server(conns)
		if err != nil {
			return
		}
	}()

	for {
		reader(conns)
	}

}

func server(conns map[quic.Connection]struct{}) error {

	// conns := make(map[quic.Connection]struct{})

	listener, err := quic.ListenAddr(addr, generateTLSConfig(), nil)
	if err != nil {
		return err
	}

	for {
		conn, err := listener.Accept(context.Background())
		if err != nil {
			return err
		}

		conns[conn] = struct{}{}
	}
}

func reader(conns map[quic.Connection]struct{}) {

	for c := range conns {
		m, err := c.ReceiveMessage(context.Background())
		if err != nil {
			log.Println("reader: ", err)
			continue
		}

		if len(m) > 0 {
			fmt.Println("message from -", c.RemoteAddr(), ": ", string(m))
			for co := range conns {
				err := co.SendMessage(m)
				if err != nil {
					log.Println("reader - send: ", err)
					continue
				}
			}
		}
	}
	//stream, err := conn.AcceptStream(context.Background())
	//if err != nil {
	//
	//}

}

// Setup a bare-bones TLS config for the server
func generateTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"quic-echo-example"},
	}
}
