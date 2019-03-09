package main

import (
	"bufio"
	"crypto/rand"
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	host "github.com/libp2p/go-libp2p-host"
	libp2p "github.com/libp2p/go-libp2p"
	crypto "github.com/libp2p/go-libp2p-crypto"
	inet "github.com/libp2p/go-libp2p-net"
	multiaddr "github.com/multiformats/go-multiaddr"
	protocol "github.com/libp2p/go-libp2p-protocol"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	discovery "github.com/libp2p/go-libp2p/p2p/discovery"
	pubsub "github.com/libp2p/go-libp2p-pubsub"

	//"github.com/gogo/protobuf/proto"
)

type discoveryNotifee struct {
	ctx context.Context
	host host.Host
	PeerChan chan pstore.PeerInfo
}

//interface to be called when new peer is found
func (n *discoveryNotifee) HandlePeerFound(pi pstore.PeerInfo) {
	n.PeerChan <- pi
}

//Initialize the MDNS service
func initMDNS(ctx context.Context, peerhost host.Host, rendezvous string) chan pstore.PeerInfo {
	// TODO: why discovery slow :/
	ser, err := discovery.NewMdnsService(ctx, peerhost, time.Second, rendezvous)
	if err != nil {
		panic(err)
	}

	//register with service so that we get notified about peer discovery
	n := &discoveryNotifee{
		ctx,
		peerhost,
		make(chan pstore.PeerInfo),
	}

	ser.RegisterNotifee(n)
	return n.PeerChan
}

func handleStream(stream inet.Stream) {
	//fmt.Println("Got a new stream!")

	// Create a buffer stream for non blocking read and write.
	rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))

	go readData(rw)
	go writeData(rw)
}

func readData(rw *bufio.ReadWriter) {
	for {
		str, err := rw.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading from buffer")
			//panic(err)
		}

		if str == "" {
			return
		}
		if str != "\n" {
			// Green console colour: 	\x1b[32m
			// Reset console colour: 	\x1b[0m
			fmt.Printf("\x1b[32m%s\x1b[0m> ", str)
		}

	}
}

func writeData(rw *bufio.ReadWriter) {
	stdReader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("> ")
		sendData, err := stdReader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading from stdin")
			panic(err)
		}

		_, err = rw.WriteString(fmt.Sprintf("%s\n", sendData))
		if err != nil {
			fmt.Println("Error writing to buffer")
			panic(err)
		}
		err = rw.Flush()
		if err != nil {
			fmt.Println("Error flushing buffer")
			panic(err)
		}
	}
}

func buildOpts(port int, priv crypto.PrivKey) []libp2p.Option {
	ip := "0.0.0.0"

    sourceMultiAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%d", ip, port))
    if err != nil {
            log.Fatal(err)
    }

	return []libp2p.Option{
		libp2p.Identity(priv),
		libp2p.ListenAddrs(sourceMultiAddr),
		libp2p.EnableRelay(),
	}
}

func joinRoom(ps *pubsub.PubSub, topic string) (*pubsub.Subscription, error) {
	return ps.Subscribe(topic)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("need to supply port: ./chat-room port")
		os.Exit(0)
	}

	topic := "main"
	if len(os.Args) == 3 {
		topic = os.Args[2]
	}

	// flags
	listenPort, err := strconv.Atoi(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	// generate random key
    r := rand.Reader
    priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, r)
    if err != nil {
            log.Fatal(err)
    }

	// create libp2p host
	host, err := libp2p.New(ctx, buildOpts(listenPort, priv)...)
	if err != nil {
		log.Fatal(err)
	}

	// add key to host peerstore for signing of messages
	//kb := &pstore.KeyBook{}
	//host.Peerstore.KeyBook.AddPrivKey(priv)

	// set pid and setup host
	pid := protocol.ID(fmt.Sprintf("/chat/1.1.0/%s", topic))
	host.SetStreamHandler(pid, handleStream)

	fmt.Printf("subscribed to topic %s\n", topic)

	fmt.Printf("multiaddr: /ip4/0.0.0.0/tcp/%v/p2p/%s\n", listenPort, host.ID().Pretty())

	// start mDNS discovery
	pchan := initMDNS(ctx, host, "noot")

	// start pubsub 
	//rt := &pubsub.RandomSubRouter{}
	// ps, err := pubsub.NewRandomSub(ctx, host) 
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// // join main room
	// sub, err := ps.Subscribe("main")

	// go func() {
	// 	for {
	// 		ps.Publish("main", []byte(fmt.Sprintf("hello from %s", host.ID())))
	// 		time.Sleep(time.Second)
	// 	}
	// }()

	// go func() {
	// 	defer sub.Cancel()

	// 	var msg *pubsub.Message
	// 	var err error

	// 	// Recover from any panic as part of the receive p2p msg process.
	// 	defer func() {
	// 		if r := recover(); r != nil {
	// 			fmt.Println(msg)
	// 			// log.WithFields(logrus.Fields{
	// 			// 	"r":        r,
	// 			// 	"msg.Data": attemptToConvertPbToString(msg.Data, message),
	// 			// }).Error("P2P message caused a panic! Recovering...")
	// 		}
	// 	}()

	// 	for {
	// 		msg, err = sub.Next(ctx)

	// 		if ctx.Err() != nil {
	// 			fmt.Println("context error:", ctx.Err())
	// 			return
	// 		}
	// 		if err != nil {
	// 			fmt.Printf("Failed to get next message: %v\n", err)
	// 			continue
	// 		}

	// 		if msg == nil || msg.GetFrom() == host.ID() {
	// 			continue
	// 		}

	// 		// d := proto.Clone(message)
	// 		// if err := proto.Unmarshal(msg.Data, d); err != nil {
	// 		// 	fmt.Println("Failed to decode data", err)
	// 		// 	continue
	// 		// }

	// 		//handler(d, msg.GetFrom())
	// 	}
	// }()

	// get notified when peer connects
	peer := <-pchan
	fmt.Printf("found peer: %s connecting...\n", peer.ID)

	host.Peerstore().AddAddrs(peer.ID, peer.Addrs, pstore.PermanentAddrTTL)

	// open stream with peer
	stream, err := host.NewStream(ctx, peer.ID, pid)
	if err != nil {
		fmt.Printf("stream err: %s\n", err)
	} else {
		rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))
		go writeData(rw)
		go readData(rw)
		fmt.Println("connected to:", peer.ID)
	}

	// add peer to pubsub router
	//ps.AddPeer(peer.ID, pid)

	select{}
}
