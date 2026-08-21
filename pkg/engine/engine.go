package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/notnil/chess"
	"github.com/sirupsen/logrus"
	"github.com/thinktt/yowapi/pkg/events"
	"github.com/thinktt/yowapi/pkg/models"
	"github.com/thinktt/yowapi/pkg/moveque"
	"github.com/thinktt/yowapi/pkg/utils"
)

const defaultWorkerTag = "default"

type Move struct {
	GameID         string
	PlayerID       string
	PlayerColor    string
	Index          int
	Move           string
	WillAcceptDraw bool
}

type GetGame func(string) (models.Game2, error)
type AddMove func(string, string, models.MoveData2) error
type OfferDraw func(string, string, string) error

type flow struct {
	getGame   GetGame
	addMove   AddMove
	offerDraw OfferDraw
}

func Start(updates <-chan events.Event, getGame GetGame, addMove AddMove, offerDraw OfferDraw) error {
	engineFlow := flow{
		getGame:   getGame,
		addMove:   addMove,
		offerDraw: offerDraw,
	}

	err := moveque.StartMoveConsumers(engineFlow.handleMoveResponse)
	if err != nil {
		return err
	}

	go engineFlow.handleGameUpdates(updates)
	return nil
}

func GetDiagnosticMove(moveReq models.MoveReq) (models.MoveData, error) {
	return moveque.GetDiagnosticMove(moveReq)
}

func (f flow) handleGameUpdates(updates <-chan events.Event) {
	for event := range updates {
		if event.Type != "gameUpdate" {
			continue
		}
		if event.Game == nil {
			continue
		}

		err := requestMove(*event.Game)
		if err != nil {
			logrus.WithField("gameId", event.GameID).
				WithError(err).
				Error("unable to request engine move")
		}
	}
}

func requestMove(game models.Game2) error {
	moveReq, shouldMove, err := BuildMoveRequest(game)
	if err != nil {
		return err
	}
	if !shouldMove {
		return nil
	}

	err = moveque.PushMove(moveReq)
	if err != nil {
		return fmt.Errorf("publish engine move request: %w", err)
	}
	return nil
}

func (f flow) handleMoveResponse(response models.MoveData) error {
	log := logrus.WithFields(logrus.Fields{
		"gameId":    response.GameId,
		"index":     response.Index,
		"workerTag": response.WorkerTag,
	})

	if response.GameId == "" {
		log.Error("discarding move response with no game ID")
		return nil
	}

	if response.Err != nil {
		publishResponseMessage("engineError", response, *response.Err)
		log.WithField("engineError", *response.Err).Error("discarding worker move error")
		return nil
	}

	if response.Warning != nil {
		publishResponseMessage("engineWarning", response, *response.Warning)
		log.WithFields(logrus.Fields{
			"engineWarning":  *response.Warning,
			"algebraMove":    response.AlgebraMove,
			"coordinateMove": response.CoordinateMove,
		}).Warn("worker returned a move warning")
	}

	game, err := f.getGame(response.GameId)
	if err != nil {
		return fmt.Errorf("load game for move response: %w", err)
	}
	if game.ID == "" {
		log.Warn("move response references a missing game")
		return nil
	}

	engineMove, err := ParseMoveResponse(game, response)
	if err != nil {
		log.WithError(err).Error("discarding invalid engine move response")
		return nil
	}

	if engineMove.WillAcceptDraw {
		err = f.offerDraw(engineMove.GameID, engineMove.PlayerID, engineMove.PlayerColor)
		if err != nil {
			return handleGameError(err, log, "discarding invalid engine draw offer")
		}
	}

	moveData := models.MoveData2{
		Index: engineMove.Index,
		Move:  engineMove.Move,
	}
	err = f.addMove(engineMove.GameID, engineMove.PlayerID, moveData)
	if err != nil {
		return handleGameError(err, log, "discarding invalid engine move response")
	}

	return nil
}

type responseMessage struct {
	GameID    string `json:"gameId"`
	Index     int    `json:"index"`
	WorkerTag string `json:"workerTag,omitempty"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

func publishResponseMessage(eventType string, response models.MoveData, message string) {
	payload := responseMessage{
		GameID:    response.GameId,
		Index:     response.Index,
		WorkerTag: response.WorkerTag,
		Message:   message,
		Timestamp: time.Now().UnixMilli(),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		logrus.WithError(err).Error("unable to encode engine response event")
		return
	}

	events.PublishMessage(response.GameId, eventType, string(data))
}

func handleGameError(err error, log *logrus.Entry, message string) error {
	if isRetryableMoveResponseError(err) {
		return err
	}
	log.WithError(err).Error(message)
	return nil
}

// NATS retries infrastructure failures but consumes invalid game responses.
func isRetryableMoveResponseError(err error) bool {
	var httpErr *utils.HTTPError
	if !errors.As(err, &httpErr) {
		return true
	}
	return httpErr.StatusCode == http.StatusInternalServerError &&
		strings.HasPrefix(httpErr.Message, "DB Error:")
}

// BuildMoveRequest returns a worker request when the current player is an engine.
func BuildMoveRequest(game models.Game2) (models.MoveReq, bool, error) {
	moveReq := models.MoveReq{}
	if game.Winner != "pending" {
		return moveReq, false, nil
	}

	player, _ := currentEnginePlayer(game)
	if player.ID == "" {
		return moveReq, false, nil
	}

	chessGame, err := parseGame(game)
	if err != nil {
		return moveReq, false, err
	}

	uciMoves := getUCIMoves(chessGame)
	moveReq = models.MoveReq{
		Moves:     uciMoves,
		CmpName:   player.ID,
		GameId:    game.ID,
		WorkerTag: player.WorkerTag,
	}

	return moveReq, true, nil
}

// ParseMoveResponse validates a worker response against the current game and
// returns a canonical move for the game flow.
func ParseMoveResponse(game models.Game2, response models.MoveData) (Move, error) {
	engineMove := Move{}
	if response.GameId == "" {
		return engineMove, fmt.Errorf("move response has no game ID")
	}
	if response.Err != nil {
		return engineMove, fmt.Errorf("worker returned a move error: %s", *response.Err)
	}

	player, color := currentEnginePlayer(game)
	if player.ID == "" {
		return engineMove, fmt.Errorf("move response arrived when the current player is not an engine")
	}

	expectedWorkerTag := player.WorkerTag
	if expectedWorkerTag == "" {
		expectedWorkerTag = defaultWorkerTag
	}
	if response.WorkerTag != expectedWorkerTag {
		return engineMove, fmt.Errorf(
			"move response has worker tag %q, expected %q",
			response.WorkerTag,
			expectedWorkerTag,
		)
	}

	move, err := getMoveFromResponse(game, response)
	if err != nil {
		return engineMove, err
	}

	engineMove = Move{
		GameID:         game.ID,
		PlayerID:       player.ID,
		PlayerColor:    color,
		Index:          response.Index,
		Move:           move,
		WillAcceptDraw: response.WillAcceptDraw,
	}
	return engineMove, nil
}

func getMoveFromResponse(game models.Game2, response models.MoveData) (string, error) {
	move := response.CoordinateMove
	if move == "" {
		move = normalizeMove(response.AlgebraMove)
	}
	if move == "" {
		return "", fmt.Errorf("worker response has no move")
	}
	if !isCoordinateMove(move) {
		return move, nil
	}

	chessGame, err := parseGame(game)
	if err != nil {
		return "", err
	}

	return getAlgebraMove(chessGame, move)
}

func currentEnginePlayer(game models.Game2) (models.Player, string) {
	if game.TurnColor() == "white" && game.WhitePlayer.Type == "cmp" {
		return game.WhitePlayer, "white"
	}
	if game.TurnColor() == "black" && game.BlackPlayer.Type == "cmp" {
		return game.BlackPlayer, "black"
	}
	return models.Player{}, ""
}

func parseGame(game models.Game2) (*chess.Game, error) {
	chessGame := chess.NewGame()
	for index, move := range game.MoveList {
		err := chessGame.MoveStr(move)
		if err != nil {
			return nil, fmt.Errorf("error parsing game at index %d: %v", index, err)
		}
	}
	return chessGame, nil
}

func getUCIMoves(chessGame *chess.Game) []string {
	chess.UseNotation(chess.UCINotation{})(chessGame)
	moves := make([]string, 0, len(chessGame.Moves()))
	for _, move := range chessGame.Moves() {
		moves = append(moves, move.String())
	}
	return moves
}

func getAlgebraMove(chessGame *chess.Game, uciMove string) (string, error) {
	chess.UseNotation(chess.UCINotation{})(chessGame)
	err := chessGame.MoveStr(uciMove)
	if err != nil {
		return "", fmt.Errorf("error adding coordinate move to chess game: %s", err.Error())
	}

	chess.UseNotation(chess.AlgebraicNotation{})(chessGame)
	pgnFields := strings.Fields(chessGame.String())
	if len(pgnFields) < 3 {
		return "", fmt.Errorf("unable to parse algebra move from chess game: PGN too short")
	}

	return pgnFields[len(pgnFields)-2], nil
}

func isCoordinateMove(move string) bool {
	if len(move) != 4 && len(move) != 5 {
		return false
	}
	if move[0] < 'a' || move[0] > 'h' {
		return false
	}
	if move[1] < '1' || move[1] > '8' {
		return false
	}
	if move[2] < 'a' || move[2] > 'h' {
		return false
	}
	if move[3] < '1' || move[3] > '8' {
		return false
	}
	return true
}

func normalizeMove(move string) string {
	// Let the chess library determine whether the move is check or checkmate.
	move = strings.TrimRight(move, "+#")

	if strings.Contains(move, "0-0-0") {
		return "O-O-O"
	}
	if strings.Contains(move, "0-0") {
		return "O-O"
	}

	for index := 1; index < len(move); index++ {
		if strings.ContainsRune("QNRB", rune(move[index])) {
			return move[:index] + "=" + move[index:]
		}
	}
	return move
}
