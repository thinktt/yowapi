package moveque

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"
	"github.com/thinktt/yowapi/pkg/models"
)

var log = logrus.New()
var moveStream nats.JetStreamContext
var nc *nats.Conn

const moveReqStreamName = "move-req-stream"
const moveResStreamName = "move-res-stream"
const moveReqSubject = "move-req"
const defaultWorkerTag = "default"
const diagnosticMoveTimeout = 15 * time.Minute

var moveReqStreamSubjects = []string{moveReqSubject + ".*"}
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

	nc, err = nats.Connect(natsUrl, nats.Token(token))
	if err != nil {
		log.Fatalf("Error connecting to NATS: %v", err)
	}

	// Create a JetStream Context
	js, err := nc.JetStream()
	if err != nil {
		log.Fatalf("Error creating JetStream context: %v", err)
	}

	err = ensureStream(js, moveReqStreamName, moveReqStreamSubjects)
	if err != nil {
		log.Fatalf("Failed to create or update %s: %v", moveReqStreamName, err)
	}
	log.Println("move-req-stream found or created")

	err = ensureStream(js, moveResStreamName, []string{"move-res.*"})
	if err != nil {
		log.Fatalf("Failed to create or update %s: %v", moveResStreamName, err)
	}
	log.Println("move-res-stream found or created")

	moveStream = js
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

func getMoveReqSubject(moveReq models.MoveReq) string {
	return fmt.Sprintf("%s.%s", moveReqSubject, moveReq.WorkerTag)
}

func withDefaultWorkerTag(moveReq models.MoveReq) models.MoveReq {
	if moveReq.WorkerTag == "" {
		moveReq.WorkerTag = defaultWorkerTag
	}
	return moveReq
}

// GetDiagnosticMove preserves the admin move-request diagnostic. Normal game
// traffic uses PushMove and the durable response consumer instead.
func GetDiagnosticMove(moveReq models.MoveReq) (models.MoveData, error) {
	moveRes := models.MoveData{}
	moveReq = withDefaultWorkerTag(moveReq)
	data, err := json.Marshal(moveReq)
	if err != nil {
		return moveRes, err
	}

	subject := fmt.Sprintf("move-res.%s", moveReq.WorkerTag)
	sub, err := nc.SubscribeSync(subject)
	if err != nil {
		return moveRes, err
	}
	defer sub.Unsubscribe()

	if _, err = moveStream.Publish(getMoveReqSubject(moveReq), data); err != nil {
		return moveRes, err
	}
	deadline := time.Now().Add(diagnosticMoveTimeout)
	for {
		msg, err := sub.NextMsg(time.Until(deadline))
		if err != nil {
			return moveRes, err
		}
		if err := json.Unmarshal(msg.Data, &moveRes); err != nil {
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
	moveReq = withDefaultWorkerTag(moveReq)
	data, err := json.Marshal(moveReq)
	if err != nil {
		return err
	}

	reqSubject := getMoveReqSubject(moveReq)
	log.WithFields(logrus.Fields{
		"gameId":    moveReq.GameId,
		"workerTag": moveReq.WorkerTag,
		"subject":   reqSubject,
	}).Debug("pushing move request")

	_, err = moveStream.Publish(reqSubject, data)
	return err
}

// StartMoveConsumers starts one durable processing loop for each
// explicitly allowed worker tag. The default worker tag is always included.
func StartMoveConsumers(handler func(models.MoveData) error) error {
	workerTags, err := moveResponseWorkerTags(os.Getenv("MOVE_RESPONSE_WORKER_TAGS"))
	if err != nil {
		return err
	}

	consumerPrefix := os.Getenv("MOVE_RESPONSE_CONSUMER_PREFIX")
	if consumerPrefix == "" {
		consumerPrefix = "yowapi"
	}
	if !consumerNamePattern.MatchString(consumerPrefix) {
		return fmt.Errorf("MOVE_RESPONSE_CONSUMER_PREFIX must contain only letters, numbers, underscores, or hyphens")
	}

	for _, workerTag := range workerTags {
		subject := fmt.Sprintf("move-res.%s", workerTag)
		consumer := fmt.Sprintf("%s-%s-v1", consumerPrefix, workerTag)
		sub, err := moveStream.PullSubscribe(
			subject,
			consumer,
			nats.BindStream(moveResStreamName),
			nats.DeliverAll(),
			nats.ManualAck(),
			nats.AckWait(time.Minute),
			nats.MaxAckPending(1),
		)
		if err != nil {
			return fmt.Errorf("subscribe to %s: %w", subject, err)
		}

		log.WithFields(logrus.Fields{
			"consumer":  consumer,
			"subject":   subject,
			"workerTag": workerTag,
		}).Info("started durable move response consumer")
		go consumeMoveResponses(sub, handler)
	}
	return nil
}

func moveResponseWorkerTags(value string) ([]string, error) {
	workerTags := []string{defaultWorkerTag}
	seen := make(map[string]struct{})
	seen[defaultWorkerTag] = struct{}{}
	for _, workerTag := range strings.Split(value, ",") {
		workerTag = strings.TrimSpace(workerTag)
		if workerTag == "" {
			continue
		}
		if !consumerNamePattern.MatchString(workerTag) {
			return nil, fmt.Errorf("MOVE_RESPONSE_WORKER_TAGS contains invalid worker tag %q", workerTag)
		}
		if _, exists := seen[workerTag]; exists {
			continue
		}
		seen[workerTag] = struct{}{}
		workerTags = append(workerTags, workerTag)
	}
	return workerTags, nil
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
		if err := json.Unmarshal(msg.Data, &moveRes); err != nil {
			log.Errorf("Discarding malformed move response: %v", err)
			if ackErr := msg.Ack(); ackErr != nil {
				log.Errorf("Error acknowledging malformed move response: %v", ackErr)
			}
			continue
		}

		if err := runMoveResponseHandler(handler, moveRes); err != nil {
			log.WithFields(logrus.Fields{
				"gameId": moveRes.GameId,
				"index":  moveRes.Index,
			}).Errorf("Retrying move response after processing error: %v", err)
			if nakErr := msg.NakWithDelay(5 * time.Second); nakErr != nil {
				log.Errorf("Error negatively acknowledging move response: %v", nakErr)
			}
			continue
		}

		if err := msg.Ack(); err != nil {
			log.Errorf("Error acknowledging processed move response: %v", err)
		}
	}
}

func runMoveResponseHandler(handler func(models.MoveData) error, moveRes models.MoveData) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("panic processing move response: %v", recovered)
		}
	}()
	return handler(moveRes)
}
