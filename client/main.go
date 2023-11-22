package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"github.com/quic-go/quic-go"
	"io"
	"log"
	"os"
)

const addr = "localhost:12345"

func main() {

	// Create a new bufio reader to read input from the console
	reader := bufio.NewReader(os.Stdin)

	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic-example"},
	}

	conn, err := quic.DialAddr(context.Background(), addr, tlsConf, &quic.Config{
		EnableDatagrams: true,
	})
	if err != nil {
		log.Println(err)
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	c := Client{
		streams: make(chan quic.Stream, 10),
		ctx:     ctx,
		Cancel:  cancel,
		Conn:    conn,
		addr:    conn.RemoteAddr().String(),
		toSend:  make(chan []byte, 5),
	}

	c.Run()

	for {
		fmt.Print("Enter a message: ")
		// Read the input from the console
		message, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading input:", err)
			return
		}
		c.SendToClient([]byte(message))

		fmt.Println("")
	}
}

type Client struct {
	streams chan quic.Stream
	ctx     context.Context
	Cancel  context.CancelFunc
	Conn    quic.Connection
	addr    string
	// ChanMess chan<- Message
	toSend chan []byte
}

func (c *Client) acceptStream() {
	fmt.Println("TEST Client.acceptStream() started")
	defer fmt.Println("TEST Client.acceptStream() Finished")

	for {
		select {
		case <-c.ctx.Done():
			break
		default:
			fmt.Println("TEST Client.acceptStream() default started")
			stream, err := c.Conn.AcceptStream(context.Background())
			if err != nil {
				log.Println("client acceptStream err: ", err)
				if err == io.EOF {
					break
				}
			}
			fmt.Println("TEST: acceptStream - ", stream.StreamID())

			c.streams <- stream
		}
	}
}

func (c *Client) Run() {
	fmt.Println("TEST Client.Run()")
	go c.acceptStream()
	go c.workWithStreams()
	go c.readDG()
	fmt.Println("TEST Client.Run() Finished")
}

func (c *Client) SendToClient(data []byte) {
	c.toSend <- data
}

func (c *Client) workWithStreams() {
	fmt.Println("TEST: workWithStreams, started")
	defer fmt.Println("TEST: workWithStreams, finished")
	buff := make([]byte, 1500)

	for {
		select {
		case <-c.ctx.Done():
			break
		case mes := <-c.toSend:
			if len(c.streams) == 0 {
				c.openStream()
			}

			stream := <-c.streams
			fmt.Println("TEST: workWithStreams, message sent - ", mes)

			_, err := stream.Write(mes) // c.streams[0].Write(mes)
			if err != nil {
				log.Println("TEST: workWithStreams, message sent error - ", err)
				c.toSend <- mes
			} else {
				c.streams <- stream
			}

		default:
			buff = buff[:0]
			if len(c.streams) == 0 {
				c.openStream()
			}

			stream := <-c.streams
			read, err := stream.Read(buff)

			if read > 0 {
				fmt.Println("From Server: ", buff[:read])
				// c.ChanMess <- newMessage(c.addr, buff[:read])
			}

			if err != nil {
				log.Println("TEST: workWithStreams, message read error - ", err)
			} else {
				c.streams <- stream
			}
		}
	}
}

func (c *Client) openStream() {
	stream, err := c.Conn.OpenStreamSync(c.ctx)
	fmt.Println("Blocked")
	if err != nil {
		log.Println("openStream: ", err)
	}

	c.streams <- stream

}

func (c *Client) readDG() {
	defer func() {
		c.Cancel()
		fmt.Println("TEST Client.readDG() Finished")
	}()
	fmt.Println("TEST Client.readDG() started")

	// (*c.Conn)
	for {
		m, err := c.Conn.ReceiveMessage(c.ctx)

		if len(m) > 0 {
			// c.ChanMess <- newMessage(c.addr, m)
			fmt.Println("From Server: ", c.addr, m)
		}

		if err != nil {
			log.Println("reader: ", err)
			break
		}
	}
}
