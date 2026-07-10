package models

import (
	"encoding/json"
	"testing"
)

func TestIsUsersTurn(t *testing.T) {
	// Sample player IDs
	whitePlayerID := "mrWhite"
	blackPlayerID := "mrBlack"

	// Mock Game2 with white's turn
	gameWhiteTurn := &Game2{
		WhitePlayer: Player{ID: whitePlayerID},
		BlackPlayer: Player{ID: blackPlayerID},
	}

	// Mock Game2 with black's turn
	gameBlackTurn := &Game2{
		WhitePlayer: Player{ID: whitePlayerID},
		BlackPlayer: Player{ID: blackPlayerID},
		MoveList:    []string{"e4"},
	}

	tests := []struct {
		game     *Game2
		userID   string
		expected bool
		testName string
	}{
		{gameWhiteTurn, whitePlayerID, true, "White player, white's turn"},
		{gameWhiteTurn, blackPlayerID, false, "Black player, white's turn"},
		{gameBlackTurn, blackPlayerID, true, "Black player, black's turn"},
		{gameBlackTurn, whitePlayerID, false, "White player, black's turn"},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			result := tt.game.IsUsersTurn(tt.userID)
			if result != tt.expected {
				t.Errorf("got %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestPlayerWorkerTagJSON(t *testing.T) {
	untaggedPlayer := Player{ID: "Wizard", Type: "cmp"}
	untaggedJSON, err := json.Marshal(untaggedPlayer)
	if err != nil {
		t.Fatal(err)
	}
	if string(untaggedJSON) != `{"id":"Wizard","type":"cmp"}` {
		t.Fatalf("got %s", untaggedJSON)
	}

	taggedPlayer := Player{ID: "Wizard", Type: "cmp", WorkerTag: "kingNT"}
	taggedJSON, err := json.Marshal(taggedPlayer)
	if err != nil {
		t.Fatal(err)
	}
	if string(taggedJSON) != `{"id":"Wizard","type":"cmp","workerTag":"kingNT"}` {
		t.Fatalf("got %s", taggedJSON)
	}
}
