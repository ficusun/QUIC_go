package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"github.com/quic-go/quic-go"
	"log"
	"os"
)

const addr = "localhost:4242"

func main() {

	// Create a new bufio reader to read input from the console
	reader := bufio.NewReader(os.Stdin)

	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic-echo-example"},
	}

	conn, err := quic.DialAddr(context.Background(), addr, tlsConf, &quic.Config{
		EnableDatagrams: true,
	})
	if err != nil {
		log.Println(err)
		panic(err)
	}

	go func() {
		for {
			message, err := conn.ReceiveMessage(context.Background())
			if err != nil {
				return
			}

			fmt.Printf("recived: %s \n", message)
		}
	}()

	for {
		fmt.Print("Enter a message: ")
		// Read the input from the console
		message, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading input:", err)
			return
		}

		err = conn.SendMessage([]byte(message))
		if err != nil {
			log.Println(err)
			return
		}
	}

	// fmt.Printf("You entered: %s", message)

}
