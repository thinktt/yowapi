package games

import (
	"testing"

	"github.com/notnil/chess"
)

func TestGetAlgebraMoveFromChessGameCheckmate(t *testing.T) {
	fen, err := chess.FEN("2Q4R/6k1/2R5/4K3/7p/7P/6P1/5q2 w - - 0 88")
	if err != nil {
		t.Fatalf("chess.FEN() error = %v", err)
	}
	chessGame := chess.NewGame(fen)

	move, err := getAlgebraMoveFromChessGame(chessGame, "c8g8")
	if err != nil {
		t.Fatalf("getAlgebraMoveFromChessGame() error = %v", err)
	}
	if move != "Qg8#" {
		t.Errorf("getAlgebraMoveFromChessGame() = %q, want %q", move, "Qg8#")
	}
}
