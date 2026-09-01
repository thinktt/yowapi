package moveque

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"
	"github.com/thinktt/yowapi/pkg/models"
)

var log = logrus.New()
var jetStream nats.JetStreamContext
var nc *nats.Conn
var responseSubscription *nats.Subscription
var apiTag string
var responseSubject string
var responseConsumerName string

const (
	requestStreamName     = "move-req-stream"
	responseStreamName    = "move-res-stream"
	diagnosticMoveTimeout = 15 * time.Minute
)

var consumerNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

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

	apiTag = os.Getenv("API_TAG")
	if apiTag == "" {
		apiTag = "default"
	}
	if !consumerNamePattern.MatchString(apiTag) {
		log.Fatalf("API_TAG contains invalid characters: %q", apiTag)
	}

	nc, err = nats.Connect(natsUrl, nats.Token(token))
	if err != nil {
		log.Fatalf("Error connecting to NATS: %v", err)
	}

	// Create a JetStream Context
	jetStream, err = nc.JetStream()
	if err != nil {
		log.Fatalf("Error creating JetStream context: %v", err)
	}

	err = ensureStream(jetStream, requestStreamName, []string{"move-req.*"})
	if err != nil {
		log.Fatalf("Failed to create or update %s: %v", requestStreamName, err)
	}
	log.Println("move-req-stream found or created")

	err = ensureStream(jetStream, responseStreamName, []string{"move-res.*"})
	if err != nil {
		log.Fatalf("Failed to create or update %s: %v", responseStreamName, err)
	}
	log.Println("move-res-stream found or created")

	responseSubject = fmt.Sprintf("move-res.%s", apiTag)
	responseConsumerName = fmt.Sprintf("yowapi-%s-v1", apiTag)
	responseSubscription, err = jetStream.PullSubscribe(
		responseSubject,
		responseConsumerName,
		nats.BindStream(responseStreamName),
		nats.DeliverAll(),
		nats.ManualAck(),
		nats.AckWait(time.Minute),
		nats.MaxAckPending(1),
	)
	if err != nil {
		log.Fatalf("Failed to create or bind response consumer: %v", err)
	}

}

// GetDiagnosticMove sends a test move to a worker and waits for the response.
// It listens for the response directly instead of using the normal consumer.
func GetDiagnosticMove(moveReq models.MoveReq) (models.MoveData, error) {
	moveRes := models.MoveData{}
	moveReq, data, err := prepareMoveRequest(moveReq)
	if err != nil {
		return moveRes, err
	}

	// create a new subscription to the move response subject
	diagnosticSub, err := nc.SubscribeSync(responseSubject)
	if err != nil {
		return moveRes, err
	}
	defer diagnosticSub.Unsubscribe()

	err = publishMoveRequest(moveReq, data)
	if err != nil {
		return moveRes, err
	}

	// watch for move responses, filter for the diagnostic "gameID"
	// we're looking for then respond with the full move response
	deadline := time.Now().Add(diagnosticMoveTimeout)
	for {
		msg, err := diagnosticSub.NextMsg(time.Until(deadline))
		if err != nil {
			return moveRes, err
		}

		err = json.Unmarshal(msg.Data, &moveRes)
		if err != nil {
			return moveRes, err
		}

		if moveRes.GameId == moveReq.GameId {
			return moveRes, nil
		}
	}
}

// PushMove takes a move request and sents it to the move-req NATS stream
// if it's unable to send the move the the NATS it will respond with an error
func PushMove(moveReq models.MoveReq) error {
	moveReq, data, err := prepareMoveRequest(moveReq)
	if err != nil {
		return err
	}

	return publishMoveRequest(moveReq, data)
}

// StartMoveResponseConsumer starts this API's durable response consumer.
func StartMoveResponseConsumer(handler func(models.MoveData) error) error {
	log.WithFields(logrus.Fields{
		"consumer": responseConsumerName,
		"subject":  responseSubject,
		"apiTag":   apiTag,
	}).Info("started durable move response consumer")
	go consumeMoveResponses(responseSubscription, handler)
	return nil
}

func prepareMoveRequest(moveReq models.MoveReq) (models.MoveReq, []byte, error) {
	if moveReq.WorkerTag == "" {
		moveReq.WorkerTag = "default"
	}
	moveReq.ApiTag = apiTag
	data, err := json.Marshal(moveReq)
	if err != nil {
		return moveReq, nil, err
	}

	return moveReq, data, nil
}

func publishMoveRequest(moveReq models.MoveReq, data []byte) error {
	reqSubject := fmt.Sprintf("move-req.%s", moveReq.WorkerTag)
	log.WithFields(logrus.Fields{
		"gameId":    moveReq.GameId,
		"workerTag": moveReq.WorkerTag,
		"subject":   reqSubject,
	}).Debug("pushing move request")

	_, err := jetStream.Publish(reqSubject, data)
	return err
}

func ensureStream(js nats.JetStreamContext, name string, subjects []string) error {
	streamConfig := &nats.StreamConfig{
		Name:     name,
		Subjects: subjects,
	}

	streamInfo, err := js.StreamInfo(name)
	if err == nats.ErrStreamNotFound {
		_, err = js.AddStream(streamConfig)
		return err
	}
	if err != nil {
		return err
	}

	streamConfig = &streamInfo.Config
	streamConfig.Subjects = subjects
	_, err = js.UpdateStream(streamConfig)
	return err
}

func consumeMoveResponses(sub *nats.Subscription, handler func(models.MoveData) error) {
	for {
		msgs, err := sub.Fetch(1, nats.MaxWait(time.Second))
		if err == nats.ErrTimeout {
			continue
		}
		if err != nil {
			log.Errorf("Error fetching move response: %v", err)
			time.Sleep(time.Second)
			continue
		}

		msg := msgs[0]
		var moveRes models.MoveData
		err = json.Unmarshal(msg.Data, &moveRes)
		if err != nil {
			log.Errorf("Discarding malformed move response: %v", err)
			ackErr := msg.Ack()
			if ackErr != nil {
				log.Errorf("Error acknowledging malformed move response: %v", ackErr)
			}
			continue
		}

		err = handler(moveRes)
		if err != nil {
			log.WithFields(logrus.Fields{
				"gameId": moveRes.GameId,
				"index":  moveRes.Index,
			}).Errorf("Retrying move response after processing error: %v", err)
			nakErr := msg.NakWithDelay(5 * time.Second)
			if nakErr != nil {
				log.Errorf("Error negatively acknowledging move response: %v", nakErr)
			}
			continue
		}

		err = msg.Ack()
		if err != nil {
			log.Errorf("Error acknowledging processed move response: %v", err)
		}
	}
}
