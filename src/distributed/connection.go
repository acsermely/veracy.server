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

	"github.com/acsermely/veracy.server/src/config"
	"github.com/acsermely/veracy.server/src/db"
	"github.com/acsermely/veracy.server/src/proto/github.com/acsermely/veracy.server/distributed/pb"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	"google.golang.org/protobuf/proto"
)

var Node *ContentNode
var IsUp = false
var ctx context.Context
var arriveChans map[string]chan []byte // This doesnt scale if users need the same ID
var arriveMutex sync.Mutex

const (
	NEED_BROADCAST_TOPIC                = "need-broadcast-topic"
	IMAGE_TRANSFER_PROTOCOL protocol.ID = "/permit-image-transfer/0.0.1"
)

func Connect(conf *config.AppConfig) *ContentNode {
	ctx = context.Background()

	arriveMutex.Lock()
	arriveChans = make(map[string]chan []byte)
	arriveMutex.Unlock()

	addrs := []string{"/ip4/0.0.0.0/udp/" + strconv.Itoa(conf.NodeUDP) + "/quic-v1", "/ip4/0.0.0.0/tcp/" + strconv.Itoa(conf.NodeTCP)}
	Node = NewNode(ctx, addrs, conf.Bootstrap)
	Node.Join(NEED_BROADCAST_TOPIC)
	sub, err := Node.Topics[NEED_BROADCAST_TOPIC].Subscribe()
	if err != nil {
		println("Subscription error for NEED_BROADCAST", err)
	}
	Node.h.SetStreamHandler(IMAGE_TRANSFER_PROTOCOL, handleStream)
	go listenToNeedTopic(sub)

	return Node
}

func NeedById(id string) ([]byte, error) {
	if len(id) == 0 {
		return nil, fmt.Errorf("invalid id")
	}
	arriveMutex.Lock()
	arriveChans[id] = make(chan []byte)
	arriveMutex.Unlock()
	Node.Topics[NEED_BROADCAST_TOPIC].Publish(ctx, []byte(id))

	select {
	case data := <-arriveChans[id]:
		arriveMutex.Lock()
		delete(arriveChans, id)
		arriveMutex.Unlock()
		return data, nil
	case <-time.After(5 * time.Second):
		arriveMutex.Lock()
		delete(arriveChans, id)
		arriveMutex.Unlock()
		return nil, fmt.Errorf("timeout")
	}
}

func handleStream(s network.Stream) {
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

	c, ok := arriveChans[transferData.Id]
	if !ok {
		fmt.Printf("No chanel for: %v\n", transferData.Id)
	}

	c <- transferData.Data
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
			panic(err)
		}
		w.Flush()
		s.Close()
	}
}
