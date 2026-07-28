package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/thinktt/yowapi/pkg/models"
)

func TestCheckStartMoves(t *testing.T) {
	input := models.Game2New{
		WhitePlayer: models.Player{ID: "Orin", Type: "cmp", WorkerTag: "kingNT"},
		BlackPlayer: models.Player{ID: "Orin", Type: "cmp"},
		Moves:       "e4 c5 Nf3",
	}

	moveList, err := checkStartMoves(input)
	if err != nil {
		t.Fatalf("checkStartMoves() error = %v", err)
	}

	if want := strings.Fields(input.Moves); !reflect.DeepEqual(moveList, want) {
		t.Errorf("MoveList = %v, want %v", moveList, want)
	}
}

func TestCheckStartMovesRejectsInvalidMoves(t *testing.T) {
	_, err := checkStartMoves(models.Game2New{
		WhitePlayer: models.Player{ID: "Orin", Type: "cmp"},
		BlackPlayer: models.Player{ID: "Orin", Type: "cmp"},
		Moves:       "e4 e5 e5",
	})
	if err == nil {
		t.Fatal("checkStartMoves() accepted an invalid move sequence")
	}
}

func TestCheckStartMovesRejectsEmptyMoves(t *testing.T) {
	_, err := checkStartMoves(models.Game2New{
		WhitePlayer: models.Player{ID: "Orin", Type: "cmp"},
		BlackPlayer: models.Player{ID: "Orin", Type: "cmp"},
		Moves:       "   ",
	})
	if err == nil {
		t.Fatal("checkStartMoves() accepted an empty move sequence")
	}
}

func TestCheckStartMovesRejectsFinishedGame(t *testing.T) {
	_, err := checkStartMoves(models.Game2New{
		WhitePlayer: models.Player{ID: "Orin", Type: "cmp"},
		BlackPlayer: models.Player{ID: "Orin", Type: "cmp"},
		Moves:       "f3 e5 g4 Qh4#",
	})
	if err == nil {
		t.Fatal("checkStartMoves() accepted a finished game")
	}
}
