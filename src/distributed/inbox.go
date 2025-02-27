package distributed

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
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
const MESSAGE_TIMEOUT = 10 * time.Second

// messageResponseMap holds channels for message delivery responses
var messageResponseMap = make(map[string]chan bool)
var messageResponseMutex sync.Mutex

// generateMessageID creates a random message ID
func generateMessageID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp if random fails
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

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
			go sendInboxResponse(msg.ReceivedFrom, false, pbMsg.MessageId)
			continue
		}

		// Send positive response to sender
		go sendInboxResponse(msg.ReceivedFrom, true, pbMsg.MessageId)
	}
}

func handleInboxResponse(stream network.Stream) {
	defer stream.Close()

	// Create a buffer to read the data
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

	// Send response to waiting channel if exists
	messageResponseMutex.Lock()
	if ch, exists := messageResponseMap[resp.MessageId]; exists {
		ch <- resp.Received
		close(ch)
		delete(messageResponseMap, resp.MessageId)
	}
	messageResponseMutex.Unlock()
}

func sendInboxResponse(to peer.ID, received bool, messageId string) {
	resp := &pb.InboxResponse{
		Received:  received,
		MessageId: messageId,
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

func PublishInboxMessage(user, sender, message string) (bool, error) {
	topic, ok := Node.Topics[INBOX_TOPIC]
	if !ok {
		return false, fmt.Errorf("inbox topic not initialized")
	}

	// Generate unique message ID
	messageId := generateMessageID()

	msg := &pb.InboxMessage{
		User:      user,
		Sender:    sender,
		Message:   message,
		Timestamp: time.Now().Unix(),
		MessageId: messageId,
	}

	data, err := proto.Marshal(msg)
	if err != nil {
		return false, fmt.Errorf("failed to marshal message: %w", err)
	}

	// Create response channel
	responseChan := make(chan bool)
	messageResponseMutex.Lock()
	messageResponseMap[messageId] = responseChan
	messageResponseMutex.Unlock()

	// Publish message
	err = topic.Publish(context.Background(), data)
	if err != nil {
		messageResponseMutex.Lock()
		delete(messageResponseMap, messageId)
		messageResponseMutex.Unlock()
		return false, fmt.Errorf("failed to publish message: %w", err)
	}

	// Wait for response with timeout
	select {
	case received := <-responseChan:
		// the channel is closed when the response is sent
		return received, nil
	case <-time.After(MESSAGE_TIMEOUT):
		messageResponseMutex.Lock()
		delete(messageResponseMap, messageId)
		messageResponseMutex.Unlock()
		return false, fmt.Errorf("message delivery timeout")
	}
}
