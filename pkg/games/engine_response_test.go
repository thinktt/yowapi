package games

import (
	"errors"
	"net/http"
	"testing"

	"github.com/thinktt/yowapi/pkg/models"
	"github.com/thinktt/yowapi/pkg/utils"
)

func TestCurrentEnginePlayerUsesSideToMove(t *testing.T) {
	game := models.Game2{
		WhitePlayer: models.Player{ID: "Orin", Type: "cmp", WorkerTag: "kingWC"},
		BlackPlayer: models.Player{ID: "Orin", Type: "cmp", WorkerTag: "zenRnd0"},
		MoveList:    []string{"e4"},
	}

	name, workerTag := currentEnginePlayer(game)
	if name != "Orin" || workerTag != "zenRnd0" {
		t.Fatalf("currentEnginePlayer() = (%q, %q), want (%q, %q)", name, workerTag, "Orin", "zenRnd0")
	}
}

func TestGetAlgebraMoveFromFreshGameAcceptsCoordinateBookMove(t *testing.T) {
	game := models.Game2{}
	chessGame, err := ParseGame(game)
	if err != nil {
		t.Fatal(err)
	}

	move, err := getAlgebraMoveFromChessGame(chessGame, "e2e4")
	if err != nil {
		t.Fatalf("getAlgebraMoveFromChessGame() returned error: %v", err)
	}
	if move != "e4" {
		t.Fatalf("getAlgebraMoveFromChessGame() = %q, want %q", move, "e4")
	}
}

func TestMoveResponseRetriesOnlyDatabaseHTTPError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "database error",
			err:  utils.NewHTTPError(http.StatusInternalServerError, "DB Error: unavailable"),
			want: true,
		},
		{
			name: "invalid move",
			err:  utils.NewHTTPError(http.StatusBadRequest, "invalid move index"),
			want: false,
		},
		{
			name: "malformed stored game",
			err:  utils.NewHTTPError(http.StatusInternalServerError, "Error parsing db game"),
			want: false,
		},
		{
			name: "raw infrastructure error",
			err:  errors.New("mongo unavailable"),
			want: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isRetryableMoveResponseError(test.err); got != test.want {
				t.Fatalf("isRetryableMoveResponseError() = %t, want %t", got, test.want)
			}
		})
	}
}
