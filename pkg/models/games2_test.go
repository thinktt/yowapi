package models

import (
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
	}

	// Mock TurnColor function to return "white" for white's turn
	gameWhiteTurn.TurnColor = (g *Game2) func() string { return "white" }

	// Mock TurnColor function to return "black" for black's turn
	// gameBlackTurn.TurnColor = (g *Game2) func() string { return "black" }

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
