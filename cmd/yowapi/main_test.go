package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/thinktt/yowapi/pkg/models"
)

func TestNewGameFromPosition(t *testing.T) {
	input := models.Game2FromPosition{
		WhitePlayer: models.Player{ID: "Orin", Type: "cmp", WorkerTag: "kingNT"},
		BlackPlayer: models.Player{ID: "Orin", Type: "cmp"},
		Moves:       "e4 c5 Nf3",
	}

	game, err := newGameFromPosition(input)
	if err != nil {
		t.Fatalf("newGameFromPosition() error = %v", err)
	}

	if game.Moves != "" {
		t.Errorf("Moves = %q, want empty string", game.Moves)
	}
	if want := strings.Fields(input.Moves); !reflect.DeepEqual(game.MoveList, want) {
		t.Errorf("MoveList = %v, want %v", game.MoveList, want)
	}
	if game.TurnColor() != "black" {
		t.Errorf("TurnColor() = %q, want black", game.TurnColor())
	}
	if game.Winner != "pending" {
		t.Errorf("Winner = %q, want pending", game.Winner)
	}
}

func TestNewGameFromPositionRejectsInvalidMoves(t *testing.T) {
	_, err := newGameFromPosition(models.Game2FromPosition{
		WhitePlayer: models.Player{ID: "Orin", Type: "cmp"},
		BlackPlayer: models.Player{ID: "Orin", Type: "cmp"},
		Moves:       "e4 e5 e5",
	})
	if err == nil {
		t.Fatal("newGameFromPosition() accepted an invalid move sequence")
	}
}

func TestNewGameFromPositionRejectsEmptyMoves(t *testing.T) {
	_, err := newGameFromPosition(models.Game2FromPosition{
		WhitePlayer: models.Player{ID: "Orin", Type: "cmp"},
		BlackPlayer: models.Player{ID: "Orin", Type: "cmp"},
		Moves:       "   ",
	})
	if err == nil {
		t.Fatal("newGameFromPosition() accepted an empty move sequence")
	}
}

func TestNewGameFromPositionRejectsFinishedGame(t *testing.T) {
	_, err := newGameFromPosition(models.Game2FromPosition{
		WhitePlayer: models.Player{ID: "Orin", Type: "cmp"},
		BlackPlayer: models.Player{ID: "Orin", Type: "cmp"},
		Moves:       "f3 e5 g4 Qh4#",
	})
	if err == nil {
		t.Fatal("newGameFromPosition() accepted a finished game")
	}
}
