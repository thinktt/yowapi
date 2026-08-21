package events

import "github.com/thinktt/yowapi/pkg/models"

type Event struct {
	Type    string
	GameID  string
	Game    *models.Game2
	Message string
}

type publisher struct {
	subscriptions map[*Subscription]struct{}
}

func (p *publisher) addSub(s *Subscription) {
	p.subscriptions[s] = struct{}{}
}

func (p *publisher) removeSub(s *Subscription) {
	delete(p.subscriptions, s)
}

func (p *publisher) publishGame(game models.Game2) {
	event := Event{
		Type:   "gameUpdate",
		GameID: game.ID,
		Game:   &game,
	}
	p.publish(event)
}

func (p *publisher) publishMessage(gameID, eventType, message string) {
	event := Event{Type: eventType, GameID: gameID, Message: message}
	p.publish(event)
}

func (p *publisher) publish(event Event) {
	for s := range p.subscriptions {
		if s.willAcceptAll {
			s.Channel <- event
			continue
		}

		_, exist := s.gameIDs[event.GameID]
		if exist {
			s.Channel <- event
		}
	}
}

func GameUpdate(game models.Game2) {
	pub.publishGame(game)
}

func PublishMessage(gameID, eventType, message string) {
	pub.publishMessage(gameID, eventType, message)
}

func SubscriptionCount() int {
	return len(pub.subscriptions)
}

var pub = &publisher{
	subscriptions: make(map[*Subscription]struct{}),
}

type Subscription struct {
	gameIDs       map[string]struct{}
	willAcceptAll bool
	Channel       chan Event
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
	pub.removeSub(s)
	close(s.Channel)
}

// NewSubscription filters by game ID. An empty list receives every event.
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
	pub.addSub(&sub)

	return &sub
}
