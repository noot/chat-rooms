package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	libp2p "github.com/libp2p/go-libp2p"
	crypto "github.com/libp2p/go-libp2p-crypto"
	host "github.com/libp2p/go-libp2p-host"
	inet "github.com/libp2p/go-libp2p-net"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	protocol "github.com/libp2p/go-libp2p-protocol"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	discovery "github.com/libp2p/go-libp2p/p2p/discovery"
	multiaddr "github.com/multiformats/go-multiaddr"
	//"github.com/gogo/protobuf/proto"
)

type discoveryNotifee struct {
	ctx      context.Context
	host     host.Host
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

	// get notified when peer connects
	peer := <-pchan
	fmt.Printf("found peer: %s connecting...\n", peer.ID)

	host.Peerstore().AddAddrs(peer.ID, peer.Addrs, pstore.PermanentAddrTTL)

	// open stream with peer
	stream, err := host.NewStream(ctx, peer.ID, pid)
	if err != nil {
	} else {
		rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))
		go writeData(rw)
		go readData(rw)
		fmt.Println("connected to:", peer.ID)
	}

	// add peer to pubsub router
	//ps.AddPeer(peer.ID, pid)

	select {}
}
