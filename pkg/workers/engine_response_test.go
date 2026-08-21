package workers

import (
	"errors"
	"net/http"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/thinktt/yowapi/pkg/games"
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
	chessGame, err := games.ParseGame(game)
	if err != nil {
		t.Fatal(err)
	}

	move, err := games.GetAlgebraMoveFromChessGame(chessGame, "e2e4")
	if err != nil {
		t.Fatalf("games.GetAlgebraMoveFromChessGame() returned error: %v", err)
	}
	if move != "e4" {
		t.Fatalf("games.GetAlgebraMoveFromChessGame() = %q, want %q", move, "e4")
	}
}

func TestGetMoveFromEngineResponsePrefersCoordinateMove(t *testing.T) {
	response := models.MoveData{
		GameId:         "test-game",
		Index:          3,
		WorkerTag:      "kingWC",
		CoordinateMove: "e2e4",
		AlgebraMove:    "d4",
	}

	move, err := getMoveFromEngineResponse(response, logrus.NewEntry(logrus.New()))
	if err != nil {
		t.Fatal(err)
	}
	if move != "e2e4" {
		t.Fatalf("engine move = %q, want %q", move, "e2e4")
	}
}

func TestGetMoveFromEngineResponseDoesNotChangeCoordinatePromotion(t *testing.T) {
	response := models.MoveData{
		GameId:         "test-game",
		CoordinateMove: "e7e8Q",
	}

	move, err := getMoveFromEngineResponse(response, logrus.NewEntry(logrus.New()))
	if err != nil {
		t.Fatal(err)
	}
	if move != "e7e8Q" {
		t.Fatalf("engine move = %q, want %q", move, "e7e8Q")
	}
}

func TestGetMoveFromEngineResponseSupportsLegacyAlgebraMove(t *testing.T) {
	response := models.MoveData{
		GameId:      "test-game",
		Index:       3,
		WorkerTag:   "kingWC",
		AlgebraMove: "0-0+",
	}

	move, err := getMoveFromEngineResponse(response, logrus.NewEntry(logrus.New()))
	if err != nil {
		t.Fatal(err)
	}
	if move != "O-O" {
		t.Fatalf("engine move = %q, want %q", move, "O-O")
	}
}

func TestGetMoveFromEngineResponseRejectsMissingMove(t *testing.T) {
	response := models.MoveData{GameId: "test-game"}

	_, err := getMoveFromEngineResponse(response, logrus.NewEntry(logrus.New()))
	if err == nil {
		t.Fatal("expected missing move error")
	}
}

func TestIsCoordinateMove(t *testing.T) {
	tests := []struct {
		move string
		want bool
	}{
		{move: "e2e4", want: true},
		{move: "e7e8q", want: true},
		{move: "O-O", want: false},
		{move: "e8=Q", want: false},
		{move: "d8b6", want: true},
	}

	for _, test := range tests {
		t.Run(test.move, func(t *testing.T) {
			got := isCoordinateMove(test.move)
			if got != test.want {
				t.Fatalf("isCoordinateMove(%q) = %t, want %t", test.move, got, test.want)
			}
		})
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
