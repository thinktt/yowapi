package models

import (
	"encoding/json"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
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

func TestGame2TagsJSON(t *testing.T) {
	untaggedGame := Game2{ID: "testgame", Tags: []string{}}
	untaggedJSON, err := json.Marshal(untaggedGame)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(untaggedJSON), `"tags"`) {
		t.Fatalf("empty tags should be omitted: %s", untaggedJSON)
	}
	untaggedBSON, err := bson.Marshal(untaggedGame)
	if err != nil {
		t.Fatal(err)
	}
	var untaggedDocument bson.M
	if err := bson.Unmarshal(untaggedBSON, &untaggedDocument); err != nil {
		t.Fatal(err)
	}
	if _, ok := untaggedDocument["tags"]; ok {
		t.Fatalf("empty tags should be omitted from BSON: %v", untaggedDocument)
	}

	taggedGame := Game2{ID: "testgame", Tags: []string{"test3", "halfCPU"}}
	taggedJSON, err := json.Marshal(taggedGame)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(taggedJSON), `"tags":["test3","halfCPU"]`) {
		t.Fatalf("tags missing from JSON: %s", taggedJSON)
	}
	taggedBSON, err := bson.Marshal(taggedGame)
	if err != nil {
		t.Fatal(err)
	}
	var taggedDocument bson.M
	if err := bson.Unmarshal(taggedBSON, &taggedDocument); err != nil {
		t.Fatal(err)
	}
	if _, ok := taggedDocument["tags"]; !ok {
		t.Fatalf("tags missing from BSON: %v", taggedDocument)
	}
}
