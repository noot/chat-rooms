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
)

type discoveryNotifee struct {
	ctx context.Context
	host host.Host
	PeerChan chan pstore.PeerInfo
}

//interface to be called when new  peer is found
func (n *discoveryNotifee) HandlePeerFound(pi pstore.PeerInfo) {
	n.PeerChan <- pi
}

//Initialize the MDNS service
func initMDNS(ctx context.Context, peerhost host.Host, rendezvous string) chan pstore.PeerInfo {
	// An hour might be a long long period in practical applications. But this is fine for us
	ser, err := discovery.NewMdnsService(ctx, peerhost, time.Minute, rendezvous)
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
	fmt.Println("Got a new stream!")

	// Create a buffer stream for non blocking read and write.
	rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))

	go readData(rw)
	go writeData(rw)

	// 'stream' will stay open until you close it (or the other side closes it).
}

func readData(rw *bufio.ReadWriter) {
	for {
		str, err := rw.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading from buffer")
			panic(err)
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

func buildOpts(port int) []libp2p.Option {
	ip := "0.0.0.0"

        //ctx := context.Background()
        r := rand.Reader

        priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, r)
        if err != nil {
                log.Fatal(err)
        }

        sourceMultiAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%d", ip, port))
        if err != nil {
                log.Fatal(err)
        }

	return []libp2p.Option{
		libp2p.Identity(priv),
		libp2p.ListenAddrs(sourceMultiAddr),
		libp2p.EnableRelay(),
		libp2p.NATPortMap(),
	}
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("need to supply host and port: ./chat-room host port")
		os.Exit(0)
	}

	listenHost := os.Args[1]
	listenPort, err := strconv.Atoi(os.Args[2])
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
/*	r := rand.Reader

	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, r)
	if err != nil {
		log.Fatal(err)
	}

	sourceMultiAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%d", listenHost, listenPort))
	if err != nil {
		log.Fatal(err)
	}

	host, err := libp2p.New(ctx, libp2p.ListenAddrs(sourceMultiAddr), libp2p.Identity(priv))*/
	host, err := libp2p.New(ctx, buildOpts(listenPort)...)
	if err != nil {
		log.Fatal(err)
	}

	pid := protocol.ID("/chat/1.1.0")
	host.SetStreamHandler(pid, handleStream)

	fmt.Printf("multiaddr: /ip4/%s/tcp/%v/p2p/%s\n", listenHost, listenPort, host.ID().Pretty())

	pchan := initMDNS(ctx, host, "noot")
	peer := <-pchan
	fmt.Printf("found peer: %s\n connecting...\n", peer.ID)
	host.Peerstore().AddAddrs(peer.ID, peer.Addrs, pstore.PermanentAddrTTL)

	stream, err := host.NewStream(ctx, peer.ID, pid)
	if err != nil {
		log.Fatalf("stream err: %s", err)
	}

	rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))
	go writeData(rw)
	go readData(rw)
	fmt.Println("connected to:", peer.ID)

	select{}
}
