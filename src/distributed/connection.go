package distributed

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	"gitlab.com/acsermely/permit-v0/server/src/config"
	"gitlab.com/acsermely/permit-v0/server/src/db"
)

var Node *ContentNode
var IsUp = false
var ctx context.Context
var arriveChans map[string]chan []byte

const (
	NEED_BROADCAST_TOPIC                = "need-broadcast-topic"
	IMAGE_TRANSFER_PROTOCOL protocol.ID = "/permit-image-transfer/0.0.1"
)

func Connect(conf *config.AppConfig) *ContentNode {
	ctx = context.Background()

	arriveChans = make(map[string]chan []byte)

	addrs := []string{"/ip4/0.0.0.0/udp/8078/quic-v1", "/ip4/0.0.0.0/tcp/8079"}
	Node = NewNode(ctx, addrs, conf.Bootstrap)
	Node.Join(NEED_BROADCAST_TOPIC)
	sub, err := Node.Topics[NEED_BROADCAST_TOPIC].Subscribe()
	if err != nil {
		println("Subscription error for NEED_BROADCAST", err)
	}
	go listenToNeedTopic(sub)

	return Node
}

func NeedById(id string) ([]byte, error) {
	if len(id) == 0 {
		return nil, fmt.Errorf("invalid id")
	}
	arriveChans[id] = make(chan []byte)
	Node.h.SetStreamHandler(IMAGE_TRANSFER_PROTOCOL, func(s network.Stream) {
		handleStream(s, id)
	})
	Node.Topics[NEED_BROADCAST_TOPIC].Publish(ctx, []byte(id))

	defer Node.h.RemoveStreamHandler(IMAGE_TRANSFER_PROTOCOL)

	select {
	case data := <-arriveChans[id]:
		delete(arriveChans, id)
		return data, nil
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("timeout")
	}
}

func handleStream(s network.Stream, id string) {
	r := bufio.NewReader(s)

	data := []byte{}
	for {
		buffer := make([]byte, 20480)
		n, err := r.Read(buffer)
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Println("Error reading from file:", err)
			return
		}
		data = append(data, buffer[:n]...)
	}
	c, ok := arriveChans[id]

	if !ok {
		fmt.Printf("No chanel for: %v\n", id)
	}
	c <- data
	close(c)
}

func listenToNeedTopic(sub *pubsub.Subscription) {
	for {
		m, err := sub.Next(ctx)
		if err != nil {
			panic(err)
		}
		from := m.ReceivedFrom.String()
		if from == Node.ID() {
			continue
		}
		fmt.Println("Distributed: Need Id")
		id := string(m.Message.Data)
		// check id in db
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

		s, err := Node.h.NewStream(ctx, m.ReceivedFrom, IMAGE_TRANSFER_PROTOCOL)
		if err != nil {
			continue
		}
		w := bufio.NewWriter(s)
		_, err = w.Write(imageData)
		if err != nil {
			panic(err)
		}
		w.Flush()
		s.Close()
	}
}
