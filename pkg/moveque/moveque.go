package moveque

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"
	"github.com/thinktt/yowapi/pkg/models"
)

var log = logrus.New()
var moveStream nats.JetStreamContext
var nc *nats.Conn

func init() {
	var err error

	token := os.Getenv("NATS_TOKEN")
	if token == "" {
		log.Fatal("NATS_TOKEN environment variable is not set")
	}

	natsUrl := os.Getenv("NATS_URL")
	if natsUrl == "" {
		log.Println("NATS_URL not set, using:", nats.DefaultURL)
		natsUrl = nats.DefaultURL
	} else {
		log.Println("NATS_URL set to:", natsUrl)
	}

	nc, err = nats.Connect(natsUrl, nats.Token(token))
	if err != nil {
		log.Fatalf("Error connecting to NATS: %v", err)
	}

	// Create a JetStream Context
	js, err := nc.JetStream()
	if err != nil {
		log.Fatalf("Error creating JetStream context: %v", err)
	}

	// Create move-req-stream
	_, err = js.AddStream(&nats.StreamConfig{
		Name:     "move-req-stream",
		Subjects: []string{"move-req"},
	})
	if err != nil {
		log.Printf("Failed to create stream: %v", err)
	} else {
		log.Println("move-req-stream found or created")
	}

	// Create move-res-stream
	_, err = js.AddStream(&nats.StreamConfig{
		Name:     "move-res-stream",
		Subjects: []string{"move-res.*"},
	})
	if err != nil {
		log.Printf("Failed to create stream: %v", err)
	} else {
		log.Println("move-res-stream found or created")
	}

	moveStream = js
}

// GetMove works with NATS in a request and response fashion, it sends a
// move to NATS and waits to hear back a response from that particular move
// it eventually fails or times out, it can be used to treat the king workers
// and NATS pub sub like an old school server
func GetMove(moveReq models.MoveReq) (models.MoveData, error) {
	moveRes := models.MoveData{}

	// Serialize moveReq to JSON
	data, err := json.Marshal(moveReq)
	if err != nil {
		return moveRes, err
	}

	subject := fmt.Sprintf("move-res.%s", moveReq.GameId)

	// Set up a subscription
	sub, err := nc.SubscribeSync(subject)
	if err != nil {
		log.Errorf("Error subscribing to subject: %v", err)
		return moveRes, err
	}
	defer sub.Unsubscribe()

	// Publish it to move-req-stream
	_, err = moveStream.Publish("move-req", data)
	if err != nil {
		// log.Error(err)
		return moveRes, err
	}

	// Wait for a single message
	msg, err := sub.NextMsg(time.Second * 60) // Waits up to 10 seconds
	if err != nil {
		log.Errorf("Error receiving message: %v", err)
		return moveRes, err
	}

	// ack this message so we will not get it again
	msg.Ack()

	// Process your message here, e.g.,:
	// fmt.Printf("Received message: %s\n", msg.Data)

	// Parse the message into moveRes
	if err := json.Unmarshal(msg.Data, &moveRes); err != nil {
		log.Errorf("Error parsing message data: %v", err)
		return moveRes, err
	}

	return moveRes, nil
}

// PushMove takes a move request and sents it to the move-req NATS stream
// if it's unable to send the move the the NATS it will respond with an error
func PushMove(moveReq models.MoveReq) error {
	data, err := json.Marshal(moveReq)
	if err != nil {
		return err
	}

	_, err = moveStream.Publish("move-req", data)
	return err
}

// GetMoveResChan returns a channel that streams move responses from
// NATs and the King workers
func GetMoveResChan() (<-chan models.MoveData, error) {
	resChan := make(chan models.MoveData)
	subject := "move-res.*"

	// Subscribe to the move response subject
	sub, err := nc.Subscribe(subject, func(msg *nats.Msg) {
		var moveRes models.MoveData
		if err := json.Unmarshal(msg.Data, &moveRes); err != nil {
			log.Errorf("Error unmarshaling move response: %v", err)
			return
		}
		resChan <- moveRes
	})
	if err != nil {
		return nil, err
	}

	// Set up a handler for when the connection closes
	nc.SetClosedHandler(func(_ *nats.Conn) {
		sub.Unsubscribe()
		close(resChan)
	})

	return resChan, nil
}
