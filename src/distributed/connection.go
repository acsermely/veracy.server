package distributed

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/acsermely/veracy.server/src/common"
	"github.com/acsermely/veracy.server/src/config"
	"github.com/acsermely/veracy.server/src/db"
	"github.com/acsermely/veracy.server/src/proto/github.com/acsermely/veracy.server/distributed/pb"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	"google.golang.org/protobuf/proto"
)

type ChannelMap map[string][]chan []byte

var (
	Node *ContentNode
	ctx  context.Context

	arriveChans ChannelMap
	arriveMutex sync.Mutex

	// Dynamic Topics
	GroupBroadcastTopic string
)

const (
	NEED_CONTENT_BROADCAST_TOPIC             = "need-content-broadcast-topic"
	IMAGE_TRANSFER_PROTOCOL      protocol.ID = "/permit-image-transfer/0.0.1"
	KEY_TRANSFER_PROTOCOL        protocol.ID = "/veracy-key-transfer/0.0.1"
	NETWORK_TIMEOUT                          = 5 * time.Second
)

func Connect(conf *config.AppConfig) *ContentNode {
	ctx = context.Background()
	arriveChans = make(ChannelMap)

	addrs := []string{
		"/ip4/0.0.0.0/tcp/" + strconv.Itoa(conf.NodeTCP),
		"/ip4/0.0.0.0/udp/" + strconv.Itoa(conf.NodeUDP) + "/quic-v1",
	}

	Node = NewNode(ctx, addrs, conf.Bootstrap)

	// Need Image protocol
	needTopic, err := Node.Join(NEED_CONTENT_BROADCAST_TOPIC)
	if err != nil {
		println("Join error for NEED_BROADCAST", err)
	}
	needSub, err := needTopic.Subscribe()
	if err != nil {
		println("Subscription error for NEED_BROADCAST", err)
	}
	Node.h.SetStreamHandler(IMAGE_TRANSFER_PROTOCOL, imageTransferHandler)
	go listenToNeedContentTopic(needSub)

	// Group protocol
	if conf.Group == "" {
		GroupBroadcastTopic = common.GenerateRandomHash()
	} else {
		GroupBroadcastTopic = conf.Group
	}
	fmt.Printf("\nGroup Code: %v\n\n", GroupBroadcastTopic)

	groupTopic, err := Node.Join(GroupBroadcastTopic)
	if err != nil {
		println("Join error for NEED_BROADCAST", err)
	}
	groupSub, err := groupTopic.Subscribe()
	if err != nil {
		println("Subscription error for NEED_BROADCAST", err)
	}
	Node.h.SetStreamHandler(KEY_TRANSFER_PROTOCOL, groupKeyTransferHandler)
	go listenToGroupKeyTopic(groupSub)

	if err := initInbox(); err != nil {
		fmt.Printf("Warning: failed to initialize inbox protocol: %v\n", err)
	}

	return Node
}

func NeedById(id string) ([]byte, error) {
	if len(id) == 0 {
		return nil, fmt.Errorf("invalid id")
	}

	arriveMutex.Lock()
	if _, exists := arriveChans[id]; !exists {
		arriveChans[id] = []chan []byte{}
	}
	newChannel := make(chan []byte)
	arriveChans[id] = append(arriveChans[id], newChannel)
	arriveMutex.Unlock()
	Node.Topics[NEED_CONTENT_BROADCAST_TOPIC].Publish(ctx, []byte(id))

	select {
	case data := <-newChannel:
		arriveMutex.Lock()
		delete(arriveChans, id)
		arriveMutex.Unlock()
		return data, nil
	case <-time.After(NETWORK_TIMEOUT):
		arriveMutex.Lock()
		delete(arriveChans, id)
		arriveMutex.Unlock()
		return nil, fmt.Errorf("timeout")
	}
}

func GroupUserByAddress(address string) ([]byte, error) {
	if len(address) == 0 {
		return nil, fmt.Errorf("invalid id")
	}

	arriveMutex.Lock()
	if _, exists := arriveChans[address]; !exists {
		arriveChans[address] = []chan []byte{}
	}
	newChannel := make(chan []byte)
	arriveChans[address] = append(arriveChans[address], newChannel)
	arriveMutex.Unlock()
	Node.Topics[GroupBroadcastTopic].Publish(ctx, []byte(address))

	select {
	case data := <-newChannel:
		arriveMutex.Lock()
		delete(arriveChans, address)
		arriveMutex.Unlock()
		return data, nil
	case <-time.After(NETWORK_TIMEOUT / 2):
		arriveMutex.Lock()
		delete(arriveChans, address)
		arriveMutex.Unlock()
		return nil, fmt.Errorf("timeout")
	}
}

func imageTransferHandler(s network.Stream) {
	r := bufio.NewReader(s)

	data := []byte{}
	for {
		buffer := make([]byte, 20480)
		n, err := r.Read(buffer)
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Println("Error reading Data:", err)
			return
		}
		data = append(data, buffer[:n]...)
	}

	transferData := &pb.ImageTransferData{}
	if err := proto.Unmarshal(data, transferData); err != nil {
		fmt.Println("Error while Unmarshal", err)
	}
	if chans, exists := arriveChans[transferData.Id]; exists {
		for _, ch := range chans {
			ch <- transferData.Data
			close(ch)
		}
	}
}

func groupKeyTransferHandler(s network.Stream) {
	r := bufio.NewReader(s)

	data := []byte{}
	for {
		buffer := make([]byte, 20480)
		n, err := r.Read(buffer)
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Println("Error reading Data:", err)
			return
		}
		data = append(data, buffer[:n]...)
	}

	transferData := &pb.KeyTransferData{}
	if err := proto.Unmarshal(data, transferData); err != nil {
		fmt.Println("Error while Unmarshal", err)
	}
	if chans, exists := arriveChans[transferData.Id]; exists {
		for _, ch := range chans {
			ch <- []byte(transferData.Key)
			close(ch)
		}
	}
}

func listenToNeedContentTopic(sub *pubsub.Subscription) {
	for {
		m, err := sub.Next(ctx)
		if err != nil {
			continue
		}
		from := m.ReceivedFrom.String()
		if from == Node.ID() {
			continue
		}
		id := string(m.Message.Data)
		if len(id) < 1 {
			continue
		}
		parts := strings.Split(id, ":")
		wallet, post, idStr := parts[0], parts[1], parts[2]
		idInt, err := strconv.Atoi(idStr)
		if err != nil {
			continue
		}
		var imageData []byte
		err = db.Database.QueryRow("SELECT data FROM images WHERE id = ? AND post = ? AND wallet = ?", idInt, post, wallet).Scan(&imageData)
		if err != nil {
			continue
		}

		transferData := &pb.ImageTransferData{
			Id:   id,
			Data: imageData,
		}

		data, err := proto.Marshal(transferData)
		if err != nil {
			fmt.Println("PB Marshal error")
			fmt.Println(err)
			continue
		}

		s, err := Node.h.NewStream(ctx, m.ReceivedFrom, IMAGE_TRANSFER_PROTOCOL)
		if err != nil {
			continue
		}
		w := bufio.NewWriter(s)
		_, err = w.Write(data)
		if err != nil {
			continue
		}
		w.Flush()
		s.Close()
	}
}

func listenToGroupKeyTopic(sub *pubsub.Subscription) {
	for {
		m, err := sub.Next(ctx)
		if err != nil {
			continue
		}
		from := m.ReceivedFrom.String()
		if from == Node.ID() {
			continue
		}
		wallet := string(m.Message.Data)
		if len(wallet) < 1 {
			continue
		}

		var userKey string
		err = db.Database.QueryRow("SELECT key FROM keys WHERE wallet = ?", wallet).Scan(&userKey)
		if err != nil {
			continue
		}

		transferData := &pb.KeyTransferData{
			Id:  wallet,
			Key: userKey,
		}

		data, err := proto.Marshal(transferData)
		if err != nil {
			fmt.Println("PB Marshal error")
			fmt.Println(err)
			continue
		}

		s, err := Node.h.NewStream(ctx, m.ReceivedFrom, KEY_TRANSFER_PROTOCOL)
		if err != nil {
			continue
		}
		w := bufio.NewWriter(s)
		_, err = w.Write(data)
		if err != nil {
			continue
		}
		w.Flush()
		s.Close()
	}
}
