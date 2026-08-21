package engine

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

	player, color := currentEnginePlayer(game)
	if player.ID != "Orin" || player.WorkerTag != "zenRnd0" || color != "black" {
		t.Fatalf("currentEnginePlayer() = (%q, %q, %q), want (%q, %q, %q)",
			player.ID, player.WorkerTag, color, "Orin", "zenRnd0", "black")
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
			got := isRetryableMoveResponseError(test.err)
			if got != test.want {
				t.Fatalf("isRetryableMoveResponseError() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestBuildMoveRequestUsesCurrentEnginePlayer(t *testing.T) {
	game := models.Game2{
		ID:          "test-game",
		Winner:      "pending",
		MoveList:    []string{"e4"},
		WhitePlayer: models.Player{ID: "human", Type: "lichess"},
		BlackPlayer: models.Player{ID: "Orin", Type: "cmp", WorkerTag: "kingWC"},
	}

	moveReq, shouldMove, err := BuildMoveRequest(game)
	if err != nil {
		t.Fatal(err)
	}
	if !shouldMove {
		t.Fatal("expected an engine move request")
	}
	if moveReq.CmpName != "Orin" || moveReq.WorkerTag != "kingWC" {
		t.Fatalf("BuildMoveRequest() player = (%q, %q), want (%q, %q)",
			moveReq.CmpName, moveReq.WorkerTag, "Orin", "kingWC")
	}
	if len(moveReq.Moves) != 1 || moveReq.Moves[0] != "e2e4" {
		t.Fatalf("BuildMoveRequest() moves = %v, want [e2e4]", moveReq.Moves)
	}
}

func TestBuildMoveRequestSkipsHumanTurn(t *testing.T) {
	game := models.Game2{
		ID:          "test-game",
		Winner:      "pending",
		WhitePlayer: models.Player{ID: "human", Type: "lichess"},
		BlackPlayer: models.Player{ID: "Orin", Type: "cmp"},
	}

	_, shouldMove, err := BuildMoveRequest(game)
	if err != nil {
		t.Fatal(err)
	}
	if shouldMove {
		t.Fatal("did not expect an engine move request on a human turn")
	}
}

func TestGetAlgebraMoveFromFreshGameAcceptsCoordinateBookMove(t *testing.T) {
	game := models.Game2{}
	chessGame, err := parseGame(game)
	if err != nil {
		t.Fatal(err)
	}

	move, err := getAlgebraMove(chessGame, "e2e4")
	if err != nil {
		t.Fatalf("getAlgebraMove() returned error: %v", err)
	}
	if move != "e4" {
		t.Fatalf("getAlgebraMove() = %q, want %q", move, "e4")
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

	move, err := getMoveFromResponse(models.Game2{}, response)
	if err != nil {
		t.Fatal(err)
	}
	if move != "e4" {
		t.Fatalf("engine move = %q, want %q", move, "e4")
	}
}

func TestGetMoveFromEngineResponseDoesNotChangeCoordinatePromotion(t *testing.T) {
	response := models.MoveData{
		GameId:         "test-game",
		CoordinateMove: "e7e8Q",
	}

	move := normalizeMove(response.CoordinateMove)
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

	move, err := getMoveFromResponse(models.Game2{}, response)
	if err != nil {
		t.Fatal(err)
	}
	if move != "O-O" {
		t.Fatalf("engine move = %q, want %q", move, "O-O")
	}
}

func TestGetMoveFromEngineResponseRejectsMissingMove(t *testing.T) {
	response := models.MoveData{GameId: "test-game"}

	_, err := getMoveFromResponse(models.Game2{}, response)
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

func TestParseMoveResponseUsesDefaultWorkerTag(t *testing.T) {
	game := models.Game2{
		ID:          "test-game",
		Winner:      "pending",
		WhitePlayer: models.Player{ID: "Orin", Type: "cmp"},
		BlackPlayer: models.Player{ID: "human", Type: "lichess"},
	}
	response := models.MoveData{
		GameId:         game.ID,
		WorkerTag:      "default",
		CoordinateMove: "e2e4",
	}

	move, err := ParseMoveResponse(game, response)
	if err != nil {
		t.Fatal(err)
	}
	if move.PlayerID != "Orin" || move.Move != "e4" {
		t.Fatalf("ParseMoveResponse() = (%q, %q), want (%q, %q)",
			move.PlayerID, move.Move, "Orin", "e4")
	}
}

func TestParseMoveResponseRejectsWrongWorkerTag(t *testing.T) {
	game := models.Game2{
		ID:          "test-game",
		Winner:      "pending",
		WhitePlayer: models.Player{ID: "Orin", Type: "cmp", WorkerTag: "kingWC"},
		BlackPlayer: models.Player{ID: "human", Type: "lichess"},
	}
	response := models.MoveData{
		GameId:         game.ID,
		WorkerTag:      "zenRnd0",
		CoordinateMove: "e2e4",
	}

	_, err := ParseMoveResponse(game, response)
	if err == nil {
		t.Fatal("expected wrong worker tag error")
	}
}
