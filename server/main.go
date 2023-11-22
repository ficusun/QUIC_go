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
	"io"
	"log"
	"math/big"
	"sync"
	"time"
)

const addr = "localhost:12345"

type Message struct {
	From string
	Body []byte
}

func newMessage(addr string, body []byte) Message {
	return Message{
		From: addr,
		Body: body,
	}
}

func (m *Message) Pack() []byte {
	return []byte(m.From + ": " + string(m.Body))
}

//func (m *Message) UnPack() Message {
//	return []byte(m.From + ": " + string(m.Body))
//}

type Server struct {
	addr       string
	conns      map[quic.Connection]*Client // Client //struct{}
	quicConfig *quic.Config
	Listener   *quic.Listener
	mx         sync.Mutex
	Messages   chan Message
}

/// Client Start ///

type Client struct {
	// streams  map[quic.Stream]struct{}
	streams []quic.Stream
	// streams chan quic.Stream
	ctx    context.Context
	Cancel context.CancelFunc
	Conn   quic.Connection
	Status bool
	addr   string
	// mx       sync.Mutex
	ChanMess chan<- Message
	toSend   chan []byte
}

func (c *Client) acceptStream() {
	fmt.Println("TEST Client.acceptStream() started")
	defer fmt.Println("TEST Client.acceptStream() Finished")

	for {
		select {
		case <-c.ctx.Done():
			fmt.Println("TEST Client.acceptStream() break and finished")
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

			// c.streams <- stream
			// go c.readStream(&stream)
			// c.streams[stream] = struct{}{}
			c.streams = append(c.streams, stream)
		}
	}
}

func (c *Client) Run() {
	fmt.Println("TEST Client.Run()")
	go c.acceptStream()
	// go c.sendToClient()
	go c.workWithStreams()
	go c.readDG()
	fmt.Println("TEST Client.Run() Finished")
}

func (c *Client) SendToClient(data []byte) {
	if c.Status {
		c.toSend <- data
	}
}

func (c *Client) check() {
	c.Conn.ReceiveMessage()
}

func (c *Client) read(stream *quic.Stream) error {
	buff := make([]byte, 1500)
	read, err := (*stream).Read(buff)

	if read > 0 {
		fmt.Println("TEST: readStream - ", buff[:read])
		c.ChanMess <- newMessage(c.addr, buff[:read])
	}

	if err != nil {
		log.Println("TEST: workWithStreams, message read error - ", err)
		return err
	}

	return nil
}

func (c *Client) send(stream *quic.Stream, mess []byte) error {
	fmt.Println("TEST: workWithStreams, message sent - ", mess)

	_, err := (*stream).Write(mess) // c.streams[0].Write(mes)
	if err != nil {
		log.Println("TEST: workWithStreams, message sent error - ", err)
		c.toSend <- mess
		return err
	}
	return nil
}

func (c *Client) workWithStreams() {
	// buff := make([]byte, 1500)

	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered. Error:\n", r)
		}
	}()

	var str quic.Stream

	for {
		if len(c.streams) > 0 {
			str = c.streams[0]
		} else {
			c.openStream()
			continue
		}

		select {
		case <-c.ctx.Done():
			fmt.Println("TEST Client.workWithStreams() break and finished")
			break
		case mes := <-c.toSend:
			err := c.send(&str, mes)
			if err != nil {
				return
			}
		default:

		}
	}
	//for {
	//	select {
	//	case <-c.ctx.Done():
	//		fmt.Println("TEST Client.workWithStreams() break and finished")
	//		break
	//	case mes := <-c.toSend:
	//		if len(c.streams) == 0 {
	//			c.openStream()
	//		}
	//
	//		stream := <-c.streams
	//		fmt.Println("TEST: workWithStreams, message sent - ", mes)
	//
	//		_, err := stream.Write(mes) // c.streams[0].Write(mes)
	//		if err != nil {
	//			log.Println("TEST: workWithStreams, message sent error - ", err)
	//			c.toSend <- mes
	//		} else {
	//			c.streams <- stream
	//		}
	//	default:
	//		buff = buff[:0]
	//		if len(c.streams) == 0 {
	//			c.openStream()
	//		}
	//
	//		stream := <-c.streams
	//		//read, err := (*stream).Read(buff)
	//		read, err := stream.Read(buff)
	//
	//		if read > 0 {
	//			fmt.Println("TEST: readStream - ", buff[:read])
	//			c.ChanMess <- newMessage(c.addr, buff[:read])
	//			//	Message{
	//			//	From: c.Conn.RemoteAddr().String(),
	//			//	Body: buff[:read],
	//			//}
	//		}
	//
	//		if err != nil {
	//			log.Println("TEST: workWithStreams, message read error - ", err)
	//			//c.mx.Lock()
	//			//delete(c.streams, *stream)
	//			//c.mx.Unlock()
	//			// break
	//		} else {
	//			c.streams <- stream
	//		}
	//	}
	//}
}

//func (c *Client) readStream(stream *quic.Stream) {
//
//	buff := make([]byte, 1500)
//
//	for {
//		select {
//		case <-c.ctx.Done():
//			break
//		default:
//			buff = buff[:0]
//			read, err := (*stream).Read(buff)
//
//			if read > 0 {
//				fmt.Println("TEST: readStream - ", buff[:read])
//				c.ChanMess <- newMessage(c.addr, buff[:read])
//				//	Message{
//				//	From: c.Conn.RemoteAddr().String(),
//				//	Body: buff[:read],
//				//}
//			}
//
//			if err != nil {
//				log.Println("readStream: ", err)
//				c.mx.Lock()
//				delete(c.streams, *stream)
//				c.mx.Unlock()
//				break
//			}
//		}
//	}
//}

func (c *Client) openStream() {
	fmt.Println("TEST: openStream - Start ")
	stream, err := c.Conn.OpenStreamSync(c.ctx)
	if err != nil {
		log.Println("openStream: ", err)
	}

	c.streams <- stream
	fmt.Println("TEST: openStream - Finished ")
	//go c.readStream(&stream)
	//c.streams[stream] = struct{}{} //append(c.streams, stream)
}

//func (c *Client) sendToClient() {
//	fmt.Println("TEST Client.sendToClient() started")
//	defer fmt.Println("TEST Client.sendToClient() Finished")
//	for {
//		fmt.Println("TEST Client.sendToClient() Circle")
//
//		select {
//		case <-c.ctx.Done():
//			break
//		case mes := <-c.toSend:
//			if len(c.streams) == 0 {
//				c.openStream()
//			}
//			fmt.Println("TEST: sendToClient, message sent - ", mes)
//			_, err := c.streams[0].Write(mes)
//			if err != nil {
//				log.Println("from SendToStream: ", err)
//				c.toSend <- mes
//			}
//		}
//	}
//}

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
			c.ChanMess <- newMessage(c.addr, m)
			//	Message{
			//	From: c.Conn.RemoteAddr().String(),
			//	Body: m,
			//}
			// fmt.Println(len(s.Messages))
			fmt.Println("TEST: readDG - ", c.addr, m)
		}

		if err != nil {
			log.Println("reader: ", err)
			break
		}
	}
}

/// Client end ///

/// SERVER ///

func (s *Server) SetAddr(addr string) {
	s.addr = addr
}

func (s *Server) SetQuicConfig() {
	s.quicConfig = &quic.Config{
		HandshakeIdleTimeout: 0,
		KeepAlivePeriod:      30 * time.Second,
		Allow0RTT:            false,
		EnableDatagrams:      true,
	}
}

func (s *Server) initListener() {
	listener, err := quic.ListenAddr(s.addr, generateTLSConfig(), s.quicConfig)
	if err != nil {
		log.Println(err)
		panic(1)
	}

	s.Listener = listener
	fmt.Println("Server started")
}

func (s *Server) ClientSpawn(conn quic.Connection) {
	s.mx.Lock()
	defer s.mx.Unlock()

	fmt.Println("TEST Server.ClientSpawn() started")
	ctx, cancel := context.WithCancel(context.Background())
	s.conns[conn] = &Client{
		// streams:  make(map[quic.Stream]struct{}),
		streams: make(chan quic.Stream, 10),
		ctx:     ctx,
		Cancel:  cancel,
		Conn:    conn,
		// mx:       sync.Mutex{},
		addr:     conn.RemoteAddr().String(),
		ChanMess: s.Messages,
		toSend:   make(chan []byte, 10),
	} // struct{}{}
	s.conns[conn].Run()
	// ctx, cancel := context.WithCancel(context.Background())
	// remove conn when it's done

	fmt.Println("Client added: ", conn.RemoteAddr().String())

	go func() {
		ctx := ctx
		conn := conn

		//_ = <-ctx.Done()
		<-ctx.Done()

		s.mx.Lock()
		defer s.mx.Unlock()

		log.Println(conn.RemoteAddr(), "removed")
		delete(s.conns, conn)
	}()

	fmt.Println("TEST Server.ClientSpawn() Finished")
}

func (s *Server) Customs() {
	fmt.Println("Customs started")
	for {
		fmt.Println("TEST Server.Customs() start waiting new client")
		conn, err := s.Listener.Accept(context.Background())
		if err != nil {
			log.Println(err)
			continue
		}
		s.ClientSpawn(conn)

		//s.mx.Lock()
		//ctx, cancel := context.WithCancel(context.Background())
		//s.conns[conn] = &Client{
		//	streams:  make([]quic.Stream, 0),
		//	ctx:      ctx,
		//	Cancel:   cancel,
		//	Conn:     &conn,
		//	ChanMess: s.Messages,
		//	toSend:   make(chan []byte, 10),
		//} // struct{}{}
		//s.conns[conn].Run()
		//// ctx, cancel := context.WithCancel(context.Background())
		//// remove conn when it's done
		//go func() {
		//	ctx := ctx
		//	conn := conn
		//
		//	//_ = <-ctx.Done()
		//	<-ctx.Done()
		//
		//	s.mx.Lock()
		//	defer s.mx.Unlock()
		//
		//	log.Println(conn.RemoteAddr(), "removed")
		//	delete(s.conns, conn)
		//}()

		//// start read
		//go func() {
		//	ctx := ctx
		//	conn := conn
		//
		//	defer cancel()
		//	for {
		//		m, err := conn.ReceiveMessage(ctx)
		//
		//		if len(m) > 0 {
		//			s.Messages <- Message{
		//				From: conn.RemoteAddr().String(),
		//				Body: m,
		//			}
		//			// fmt.Println(len(s.Messages))
		//			fmt.Println("read : ", s.addr, m)
		//		}
		//
		//		if err != nil {
		//			log.Println("reader: ", err)
		//			break
		//		}
		//	}
		//}()

		//go func() {
		//	conn := conn
		//	ctx := ctx
		//	fmt.Println("enter to the stream func")
		//
		//	for {
		//		fmt.Println("start waiting stream")
		//		str, err := conn.AcceptStream(ctx)
		//		fmt.Println("stream accepted : ", str.StreamID())
		//		mess := make([]byte, 0, 1500)
		//		for {
		//			// fmt.Println("reading from stream")
		//			mess = mess[:0]
		//			m, err := str.Read(mess)
		//
		//			if m > 0 {
		//				fmt.Println("read : ", conn.RemoteAddr(), mess[:m])
		//				s.Messages <- Message{
		//					From: conn.RemoteAddr().String(),
		//					Body: mess[:m],
		//				}
		//				// fmt.Println(len(s.Messages))
		//				fmt.Println("read : ", conn.RemoteAddr(), mess[:m])
		//			}
		//
		//			if err != nil {
		//				log.Println("reader from stream: ", err)
		//				break
		//			}
		//		}
		//
		//		if err != nil {
		//			log.Println("reader: ", err)
		//			break
		//		}
		//	}
		//}()

		//fmt.Println("Client added: ", conn.RemoteAddr().String())
		//s.mx.Unlock()
	}
}

//
//func (s *Server) streamConn() {
//	// fmt.Println("streamConn started")
//	s.mx.Lock()
//	defer s.mx.Unlock()
//	for c := range s.conns {
//		fmt.Println("THIS IS SPARTA")
//		stream, err := c.acceptStream(context.Background())
//		fmt.Println("Blocked")
//		if err != nil {
//			log.Println(err)
//			continue
//		}
//		s.conns[c] = stream
//		fmt.Println("stream ", stream.StreamID(), "added for: ", c.RemoteAddr().String())
//
//	}
//}
//
//func (s *Server) reader() {
//	toDel := make([]quic.Connection, 0)
//
//	for conn, stream := range s.conns {
//		m, err := conn.ReceiveMessage(context.Background())
//		if err != nil {
//			log.Println("reader: ", err)
//			toDel = append(toDel, conn)
//			continue
//		}
//
//		if stream != nil {
//			fmt.Println("reader started")
//
//			mes := make([]byte, 0)
//			count, err := stream.Read(mes)
//			if err != nil {
//				log.Println("stream.Read: ", err)
//			}
//
//			if count > 0 {
//				s.Messages <- Message{
//					From: s.addr,
//					Body: mes,
//				}
//				fmt.Println("read : ", s.addr, mes)
//			}
//		}
//
//		if len(m) > 0 {
//			s.Messages <- Message{
//				From: s.addr,
//				Body: m,
//			}
//			fmt.Println("read : ", s.addr, m)
//		}
//	}
//
//	s.mx.Lock()
//	for c := range toDel {
//		log.Println("deleted: ", toDel[c].RemoteAddr().String())
//		delete(s.conns, toDel[c])
//	}
//	s.mx.Unlock()
//	//stream, err := conn.acceptStream(context.Background())
//	//if err != nil {
//	//
//	//}
//	//
//	//if len(m) > 0 {
//	//	fmt.Println("message from -", c.RemoteAddr(), ": ", string(m))
//	//	for co := range conns {
//	//		err := co.SendMessage([]byte("from - " + c.RemoteAddr().String() + ": " + string(m)))
//	//		if err != nil {
//	//			log.Println("reader - send: ", err)
//	//			continue
//	//		}
//	//	}
//	//}
//}

func (s *Server) Sender() {
	fmt.Println("Sender started")

	for m := range s.Messages {
		fmt.Println("Sent")

		s.mx.Lock()
		for _, c := range s.conns {
			c.SendToClient(m.Pack())
			//err := c.SendMessage([]byte(m.From + ": " + string(m.Body)))
			//if err != nil {
			//	log.Println("send: ", err)
			//	continue
			//}
			fmt.Println("sent to ", m)
		}
		s.mx.Unlock()
	}
}

func (s *Server) init(addr string) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("init server was recovered: ", r)
		}
	}()
	// s.conns = make(map[quic.Connection]quic.Stream)
	s.conns = make(map[quic.Connection]*Client) // struct{}
	s.Messages = make(chan Message, 100)
	s.mx = sync.Mutex{}
	s.SetAddr(addr)
	s.SetQuicConfig()
	s.initListener()
	go s.Customs()
	s.Sender()
}

func main() {
	// conns := make(map[quic.Connection]struct{})
	defer func() {
		if r := recover(); r != nil {
			log.Println("init server was recovered: ", r)
		}
	}()

	s := Server{}
	s.init(addr)

}

// generateTLSConfig a bare-bones TLS config for the server
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
		NextProtos:   []string{"quic-example"},
	}
}
