package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/thinktt/yowapi/pkg/models"
)

func TestBuildGameFromPosition(t *testing.T) {
	input := models.Game2FromPosition{
		WhitePlayer: models.Player{ID: "Orin", Type: "cmp", WorkerTag: "kingNT"},
		BlackPlayer: models.Player{ID: "Orin", Type: "cmp"},
		Moves:       "e4 c5 Nf3",
		Tags:        []string{"test3", "openingSuite1"},
	}

	game, err := buildGameFromPosition(input)
	if err != nil {
		t.Fatalf("buildGameFromPosition() error = %v", err)
	}

	if game.Moves != "" {
		t.Errorf("Moves = %q, want empty string", game.Moves)
	}
	if want := strings.Fields(input.Moves); !reflect.DeepEqual(game.MoveList, want) {
		t.Errorf("MoveList = %v, want %v", game.MoveList, want)
	}
	if !reflect.DeepEqual(game.Tags, input.Tags) {
		t.Errorf("Tags = %v, want %v", game.Tags, input.Tags)
	}
	if game.TurnColor() != "black" {
		t.Errorf("TurnColor() = %q, want black", game.TurnColor())
	}
	if game.Winner != "pending" {
		t.Errorf("Winner = %q, want pending", game.Winner)
	}
}

func TestBuildGameFromPositionRejectsInvalidMoves(t *testing.T) {
	_, err := buildGameFromPosition(models.Game2FromPosition{
		WhitePlayer: models.Player{ID: "Orin", Type: "cmp"},
		BlackPlayer: models.Player{ID: "Orin", Type: "cmp"},
		Moves:       "e4 e5 e5",
	})
	if err == nil {
		t.Fatal("buildGameFromPosition() accepted an invalid move sequence")
	}
}

func TestBuildGameFromPositionRejectsEmptyMoves(t *testing.T) {
	_, err := buildGameFromPosition(models.Game2FromPosition{
		WhitePlayer: models.Player{ID: "Orin", Type: "cmp"},
		BlackPlayer: models.Player{ID: "Orin", Type: "cmp"},
		Moves:       "   ",
	})
	if err == nil {
		t.Fatal("buildGameFromPosition() accepted an empty move sequence")
	}
}

func TestBuildGameFromPositionRejectsFinishedGame(t *testing.T) {
	_, err := buildGameFromPosition(models.Game2FromPosition{
		WhitePlayer: models.Player{ID: "Orin", Type: "cmp"},
		BlackPlayer: models.Player{ID: "Orin", Type: "cmp"},
		Moves:       "f3 e5 g4 Qh4#",
	})
	if err == nil {
		t.Fatal("buildGameFromPosition() accepted a finished game")
	}
}
