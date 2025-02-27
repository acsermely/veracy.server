package distributed

import (
	"context"
	"fmt"
	"time"

	"github.com/acsermely/veracy.server/src/db"
	"github.com/acsermely/veracy.server/src/proto/github.com/acsermely/veracy.server/distributed/pb"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"google.golang.org/protobuf/proto"
)

const INBOX_PROTOCOL = "/inbox/1.0.0"
const INBOX_TOPIC = "inbox-messages"

func initInbox() error {
	// Join inbox topic
	topic, err := Node.Join(INBOX_TOPIC)
	if err != nil {
		return fmt.Errorf("failed to join inbox topic: %w", err)
	}

	// Subscribe to inbox messages
	sub, err := topic.Subscribe()
	if err != nil {
		return fmt.Errorf("failed to subscribe to inbox topic: %w", err)
	}

	// Handle incoming messages
	go handleInboxMessages(sub)

	// Set up direct response protocol
	Node.h.SetStreamHandler(INBOX_PROTOCOL, handleInboxResponse)

	return nil
}

func handleInboxMessages(sub *pubsub.Subscription) {
	for {
		msg, err := sub.Next(context.Background())
		if err != nil {
			fmt.Printf("Error getting next message: %v\n", err)
			continue
		}

		// Skip messages from self
		if msg.ReceivedFrom == Node.PeerID() {
			continue
		}

		var pbMsg pb.InboxMessage
		err = proto.Unmarshal(msg.Data, &pbMsg)
		if err != nil {
			fmt.Printf("Error unmarshaling message: %v\n", err)
			continue
		}

		// Check if user exists in our database
		_, err = db.GetUserKey(pbMsg.User)
		if err != nil {
			continue // User not found, ignore message
		}

		// Add message to inbox
		err = db.AddInboxMessage(pbMsg.User, pbMsg.Sender, pbMsg.Message)
		if err != nil {
			fmt.Printf("Error adding message to inbox: %v\n", err)
			continue
		}

		// Send direct response to sender
		go sendInboxResponse(msg.ReceivedFrom, true)
	}
}

func handleInboxResponse(stream network.Stream) {
	defer stream.Close()

	// Create a buffer to read the data - increased size for longer error messages
	data := make([]byte, 1024)
	n, err := stream.Read(data)
	if err != nil {
		fmt.Printf("Error reading from stream: %v\n", err)
		return
	}

	var resp pb.InboxResponse
	err = proto.Unmarshal(data[:n], &resp)
	if err != nil {
		fmt.Printf("Error reading response: %v\n", err)
		return
	}

	if !resp.Received {
		fmt.Printf("Message delivery failed\n")
	}
}

func sendInboxResponse(to peer.ID, received bool) {
	resp := &pb.InboxResponse{
		Received: received,
	}

	data, err := proto.Marshal(resp)
	if err != nil {
		fmt.Printf("Error marshaling response: %v\n", err)
		return
	}

	stream, err := Node.h.NewStream(context.Background(), to, INBOX_PROTOCOL)
	if err != nil {
		fmt.Printf("Error opening stream: %v\n", err)
		return
	}
	defer stream.Close()

	_, err = stream.Write(data)
	if err != nil {
		fmt.Printf("Error writing response: %v\n", err)
	}
}

func PublishInboxMessage(user, sender, message string) error {
	topic, ok := Node.Topics[INBOX_TOPIC]
	if !ok {
		return fmt.Errorf("inbox topic not initialized")
	}

	msg := &pb.InboxMessage{
		User:      user,
		Sender:    sender,
		Message:   message,
		Timestamp: time.Now().Unix(),
	}

	data, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	err = topic.Publish(context.Background(), data)
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}
