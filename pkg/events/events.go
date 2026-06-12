package events

type Publisher struct {
	subscriptions map[*Subscription]struct{}
}

func (p *Publisher) AddSub(s *Subscription) {
	p.subscriptions[s] = struct{}{}
}

func (p *Publisher) RemoveSub(s *Subscription) {
	delete(p.subscriptions, s)
}

func (p *Publisher) PublishMessage(gameID, msg string) {
	for s := range p.subscriptions {
		if s.willAcceptAll {
			s.Channel <- msg
			continue
		}

		_, exist := s.gameIDs[gameID]
		if exist {
			s.Channel <- msg
		}
	}
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
	Channel       chan string
	MessageCount  int
}

// PublishMessage allows you to directly publish messages ot this subscription
func (s *Subscription) PublishMessage(msg string) {
	s.Channel <- msg
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
		Channel:       make(chan string),
	}

	if len(gameIDs) == 0 {
		sub.willAcceptAll = true
		return &sub
	}

	for _, gameID := range gameIDs {
		sub.gameIDs[gameID] = struct{}{}
	}

	// Add the subscription set to the package global publisher
	Pub.AddSub(&sub)

	return &sub
}
