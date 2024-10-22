package distributed

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

type ContentNode struct {
	ctx    context.Context
	h      host.Host
	ps     *pubsub.PubSub
	Topics map[string]*pubsub.Topic
}

func NewNode(ctx context.Context, addr []string, bootstrap string) *ContentNode {
	h, err := libp2p.New(
		libp2p.ListenAddrStrings(addr...),
		libp2p.ForceReachabilityPublic(),
	)
	if err != nil {
		panic(err)
	}

	bootstrapPeer := peer.AddrInfo{}
	if bootstrap != "" {
		bootstrapPeer, _ = convertUrlToAddrInfo(&bootstrap)
	} else {
		printNewPeerInfo(h)
	}

	kademliaDHT, err := dht.New(ctx, h, dht.BootstrapPeers(bootstrapPeer))
	if err != nil {
		panic(err)
	}
	if err = kademliaDHT.Bootstrap(ctx); err != nil {
		panic(err)
	}

	if bootstrap != "" {
		peerAddr, _ := multiaddr.NewMultiaddr(bootstrap)
		peerinfo, _ := peer.AddrInfoFromP2pAddr(peerAddr)
		if err := h.Connect(ctx, *peerinfo); err != nil {
			fmt.Println("Bootstrap warning:", err)
		}
		fmt.Printf("bootstrap: %v\n", bootstrap)
	}

	ps, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		panic(err)
	}
	return &ContentNode{ctx: ctx, h: h, ps: ps, Topics: make(map[string]*pubsub.Topic)}
}

func (node *ContentNode) Join(s string) (*pubsub.Topic, error) {
	topic, err := node.ps.Join(s)
	if err != nil {
		return topic, err
	}
	node.Topics[s] = topic
	return topic, nil
}

func (node *ContentNode) ID() string {
	return node.h.ID().String()
}

func (node *ContentNode) PeerID() peer.ID {
	return node.h.ID()
}

func printNewPeerInfo(h host.Host) {
	fmt.Sprintln("Start new peers:")
	for _, ownAddress := range h.Addrs() {
		fmt.Printf("go run . -b %v/p2p/%v -p 8081\n", ownAddress, h.ID())
	}
}

func convertUrlToAddrInfo(addr *string) (peer.AddrInfo, error) {
	bootstrapAddress, err := multiaddr.NewMultiaddr(*addr)
	if err != nil {
		return peer.AddrInfo{}, err
	}
	info, err := peer.AddrInfoFromP2pAddr(bootstrapAddress)
	if err != nil {
		return peer.AddrInfo{}, err
	}
	return *info, nil
}
