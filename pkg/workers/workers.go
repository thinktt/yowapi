package workers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/thinktt/yowapi/pkg/events"
	"github.com/thinktt/yowapi/pkg/games"
	"github.com/thinktt/yowapi/pkg/models"
	"github.com/thinktt/yowapi/pkg/moveque"
	"github.com/thinktt/yowapi/pkg/utils"
)

func Start() error {
	return moveque.StartMoveResponseConsumers(HandleMoveResponse)
}

func GetDiagnosticMove(moveReq models.MoveReq) (models.MoveData, error) {
	return moveque.GetDiagnosticMove(moveReq)
}

func RequestMove(game models.Game2) {

	// the game is over, get out of here
	if game.Winner != "pending" {
		return
	}

	// look for any cmp playing this game
	var cmpName string
	var workerTag string
	if game.TurnColor() == "white" && game.WhitePlayer.Type == "cmp" {
		cmpName = game.WhitePlayer.ID
		workerTag = game.WhitePlayer.WorkerTag
	}
	if game.TurnColor() == "black" && game.BlackPlayer.Type == "cmp" {
		cmpName = game.BlackPlayer.ID
		workerTag = game.BlackPlayer.WorkerTag
	}

	// no cmp found for this turn, nothing needs to be done
	if cmpName == "" {
		// fmt.Println("not a cmp turn")
		return
	}

	chessGame, err := games.ParseGame(game)
	if err != nil {
		fmt.Println("Error parsing game: ", err.Error())
		return
	}

	uciMoves, err := games.GetUCIMovesFromChessGame(chessGame)
	if err != nil {
		fmt.Println("Error parsing UCI moves: ", err.Error())
		return
	}

	moveReq := models.MoveReq{
		Moves:     uciMoves,
		CmpName:   cmpName,
		GameId:    game.ID,
		WorkerTag: workerTag,
	}

	// Publish one request. The durable response consumer applies the move later.
	err = moveque.PushMove(moveReq)
	if err != nil {
		fmt.Println("Error publishing engine move request: ", err.Error())
	}
}

// HandleMoveResponse handles the engine-specific response details, then
// hands the move to the normal game move path.
func HandleMoveResponse(moveResponse models.MoveData) error {
	log := logrus.WithFields(logrus.Fields{
		"gameId":    moveResponse.GameId,
		"index":     moveResponse.Index,
		"workerTag": moveResponse.WorkerTag,
	})

	move, err := getMoveFromEngineResponse(moveResponse, log)
	if err != nil {
		log.WithError(err).Error("discarding malformed worker move response")
		return nil
	}

	game, err := games.GetGame(moveResponse.GameId)
	if err != nil {
		return fmt.Errorf("load game for move response: %w", err)
	}
	if game.ID == "" {
		log.Warn("move response references a missing game")
		return nil
	}

	cmpName, workerTag := currentEnginePlayer(game)
	if cmpName == "" {
		log.Error("move response arrived when the current player is not an engine")
		return nil
	}
	if moveResponse.WorkerTag != workerTag {
		log.WithField("expectedWorkerTag", workerTag).Error("move response has the wrong worker tag")
		return nil
	}

	move, err = getEngineMove(game, move)
	if err != nil {
		log.WithError(err).Error("cannot normalize engine move")
		return nil
	}

	if moveResponse.WillAcceptDraw {
		err = games.OfferDraw(game.ID, cmpName, game.TurnColor())
		if err != nil {
			if isRetryableMoveResponseError(err) {
				return err
			}
			log.WithError(err).Error("discarding invalid engine draw offer")
			return nil
		}
	}

	moveData := models.MoveData2{
		Index: moveResponse.Index,
		Move:  move,
	}
	err = games.AddMove(game.ID, cmpName, moveData)
	if err != nil {
		if isRetryableMoveResponseError(err) {
			return err
		}
		log.WithError(err).Error("discarding invalid engine move response")
	}

	return nil
}

func getMoveFromEngineResponse(moveResponse models.MoveData, log *logrus.Entry) (string, error) {
	if moveResponse.GameId == "" {
		return "", fmt.Errorf("move response has no game ID")
	}
	if moveResponse.Err != nil {
		publishEngineResponseEvent("engineError", moveResponse, *moveResponse.Err)
		return "", fmt.Errorf("worker returned a move error: %s", *moveResponse.Err)
	}
	if moveResponse.Warning != nil {
		publishEngineResponseEvent("engineWarning", moveResponse, *moveResponse.Warning)
		log.WithFields(logrus.Fields{
			"engineWarning":  *moveResponse.Warning,
			"algebraMove":    moveResponse.AlgebraMove,
			"coordinateMove": moveResponse.CoordinateMove,
		}).Warn("worker returned a move warning")
	}

	move := moveResponse.CoordinateMove
	if move == "" {
		move = moveResponse.AlgebraMove
		move = normalizeEngineMove(move)
	}
	if move == "" {
		return "", fmt.Errorf("worker response has no move")
	}

	return move, nil
}

type engineResponseEvent struct {
	GameID    string `json:"gameId"`
	Index     int    `json:"index"`
	WorkerTag string `json:"workerTag,omitempty"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

func publishEngineResponseEvent(event string, response models.MoveData, message string) {
	payload := engineResponseEvent{
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
	events.Pub.PublishEvent(response.GameId, event, string(data))
}

func getEngineMove(game models.Game2, move string) (string, error) {
	if !isCoordinateMove(move) {
		return move, nil
	}

	chessGame, err := games.ParseGame(game)
	if err != nil {
		return "", err
	}

	return games.GetAlgebraMoveFromChessGame(chessGame, move)
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

func currentEnginePlayer(game models.Game2) (string, string) {
	if game.TurnColor() == "white" && game.WhitePlayer.Type == "cmp" {
		return game.WhitePlayer.ID, game.WhitePlayer.WorkerTag
	}
	if game.TurnColor() == "black" && game.BlackPlayer.Type == "cmp" {
		return game.BlackPlayer.ID, game.BlackPlayer.WorkerTag
	}
	return "", ""
}

func isRetryableMoveResponseError(err error) bool {
	var httpErr *utils.HTTPError
	if !errors.As(err, &httpErr) {
		return true
	}
	return httpErr.StatusCode == http.StatusInternalServerError &&
		strings.HasPrefix(httpErr.Message, "DB Error:")
}

func normalizeEngineMove(move string) string {

	// engine edge case sometimes misreporting checkmate as check tripping up
	// chess lib, remove check and mate symbols and let the library decide
	move = strings.TrimRight(move, "+#")

	// fix weird casling notation
	if strings.Contains(move, "0-0-0") {
		return "O-O-O"
	} else if strings.Contains(move, "0-0") {
		return "O-O"
	}

	// add equal sign to promition moves
	for i := 1; i < len(move); i++ {
		if strings.ContainsRune("QNRB", rune(move[i])) {
			return move[:i] + "=" + move[i:]
		}
	}
	return move
}
