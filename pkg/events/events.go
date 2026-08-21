package events

import (
	"encoding/json"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/thinktt/yowapi/pkg/models"
)

type Event struct {
	Type    string
	GameID  string
	Game    *models.Game2
	Message string
}

type Publisher struct {
	subscriptions map[*Subscription]struct{}
}

func (p *Publisher) AddSub(s *Subscription) {
	p.subscriptions[s] = struct{}{}
}

func (p *Publisher) RemoveSub(s *Subscription) {
	delete(p.subscriptions, s)
}

func (p *Publisher) PublishGame(game models.Game2) {
	event := Event{
		Type:   "gameUpdate",
		GameID: game.ID,
		Game:   &game,
	}
	p.publish(event)
}

func (p *Publisher) PublishEvent(gameID, event, data string) {
	message := Event{Type: event, GameID: gameID, Message: data}
	p.publish(message)
}

func (p *Publisher) publish(message Event) {
	for s := range p.subscriptions {
		if s.willAcceptAll {
			s.Channel <- message
			continue
		}

		_, exist := s.gameIDs[message.GameID]
		if exist {
			s.Channel <- message
		}
	}
}

func GameUpdate(game models.Game2) {
	Pub.PublishGame(game)
}

func EngineError(response models.MoveData, message string) {
	publishEngineEvent("engineError", response, message)
}

func EngineWarning(response models.MoveData, message string) {
	publishEngineEvent("engineWarning", response, message)
}

type engineEvent struct {
	GameID    string `json:"gameId"`
	Index     int    `json:"index"`
	WorkerTag string `json:"workerTag,omitempty"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

func publishEngineEvent(event string, response models.MoveData, message string) {
	payload := engineEvent{
		GameID:    response.GameId,
		Index:     response.Index,
		WorkerTag: response.WorkerTag,
		Message:   message,
		Timestamp: time.Now().UnixMilli(),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		logrus.WithError(err).Error("unable to encode engine event")
		return
	}

	Pub.PublishEvent(response.GameId, event, string(data))
}

func (p *Publisher) GetSubCount() int {
	return len(p.subscriptions)
}

var Pub = &Publisher{
	subscriptions: make(map[*Subscription]struct{}),
}

type Subscription struct {
	gameIDs       map[string]struct{}
	willAcceptAll bool
	Channel       chan Event
	MessageCount  int
}

// PublishGame allows an initial game state to be sent directly to a subscription.
func (s *Subscription) PublishGame(game models.Game2) {
	s.Channel <- Event{Type: "gameUpdate", GameID: game.ID, Game: &game}
}

func (s *Subscription) AddGameID(gameID string) {
	s.gameIDs[gameID] = struct{}{}
}

func (s *Subscription) RemoveGameID(gameID string) {
	delete(s.gameIDs, gameID)
}

func (s *Subscription) Destroy() {
	Pub.RemoveSub(s)
	close(s.Channel)
}

// NewSubscription creates a subscription to game events. It will filter events
// by GameIDs, if GammeIDs list is empty it will all game messages will be
// published to the subscription
func NewSubscription(gameIDs []string) *Subscription {
	sub := Subscription{
		gameIDs:       make(map[string]struct{}),
		willAcceptAll: false,
		Channel:       make(chan Event),
	}

	if len(gameIDs) == 0 {
		sub.willAcceptAll = true
	} else {
		for _, gameID := range gameIDs {
			sub.gameIDs[gameID] = struct{}{}
		}
	}

	// Add the subscription set to the package global publisher
	Pub.AddSub(&sub)

	return &sub
}
