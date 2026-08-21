package events

import (
	"testing"
	"time"

	"github.com/thinktt/yowapi/pkg/models"
)

func TestSubscriptionWithNoGameIDsReceivesAllGames(t *testing.T) {
	subscription := NewSubscription([]string{})
	defer subscription.Destroy()

	game := models.Game2{ID: "testgame"}
	go GameUpdate(game)

	select {
	case message := <-subscription.Channel:
		if message.Game == nil {
			t.Fatal("game update did not include the game")
		}
		if message.Game.ID != game.ID {
			t.Fatalf("game ID = %q, want %q", message.Game.ID, game.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("all-games subscription did not receive game update")
	}
}

func TestSubscriptionFiltersGameUpdates(t *testing.T) {
	subscription := NewSubscription([]string{"wanted"})
	defer subscription.Destroy()

	go GameUpdate(models.Game2{ID: "wanted"})

	select {
	case message := <-subscription.Channel:
		if message.GameID != "wanted" {
			t.Fatalf("game ID = %q, want wanted", message.GameID)
		}
	case <-time.After(time.Second):
		t.Fatal("filtered subscription did not receive matching game update")
	}
}
