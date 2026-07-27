package games

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/thinktt/yowapi/pkg/db"
	"github.com/thinktt/yowapi/pkg/models"
	"github.com/thinktt/yowapi/pkg/moveque"
	"github.com/thinktt/yowapi/pkg/utils"
)

func PlayEngineMove(game models.Game2) {

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

	chessGame, err := ParseGame(game)
	if err != nil {
		fmt.Println("Error parsing game: ", err.Error())
		return
	}

	uciMoves, err := getUCIMovesFromChessGame(chessGame)
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

// ApplyEngineMoveResponse processes one durable worker response. Permanent
// response errors are logged and consumed; transient database errors are
// returned so NATS can redeliver the response.
func ApplyEngineMoveResponse(engineMove models.MoveData) error {
	log := logrus.WithFields(logrus.Fields{
		"gameId":    engineMove.GameId,
		"index":     engineMove.Index,
		"workerTag": engineMove.WorkerTag,
	})

	// Reject malformed worker responses that cannot become valid on retry.
	if engineMove.GameId == "" {
		log.Error("move response has no game ID")
		return nil
	}
	if engineMove.Err != nil {
		log.WithField("engineError", *engineMove.Err).Error("worker returned a move error")
		return nil
	}
	if engineMove.Warning != nil {
		log.WithFields(logrus.Fields{
			"engineWarning":  *engineMove.Warning,
			"algebraMove":    engineMove.AlgebraMove,
			"coordinateMove": engineMove.CoordinateMove,
		}).Warn("worker returned a move warning")
	}

	// Load the current game and discard responses for obsolete game states.
	game, err := db.GetGame2(engineMove.GameId)
	if err != nil {
		return fmt.Errorf("load game for move response: %w", err)
	}
	if game.ID == "" {
		log.Warn("move response references a missing game")
		return nil
	}
	if game.Winner != "pending" {
		log.Debug("discarding move response for a finished game")
		return nil
	}
	if engineMove.Index != len(game.MoveList) {
		log.WithField("currentIndex", len(game.MoveList)).Debug("discarding stale move response")
		return nil
	}

	cmpName, workerTag := currentEnginePlayer(game)
	if cmpName == "" {
		log.Error("move response arrived when the current player is not an engine")
		return nil
	}
	if engineMove.WorkerTag != workerTag {
		log.WithField("expectedWorkerTag", workerTag).Error("move response has the wrong worker tag")
		return nil
	}

	// Resolve the worker move against the current game position.
	chessGame, err := ParseGame(game)
	if err != nil {
		log.WithError(err).Error("cannot parse game for move response")
		return nil
	}
	move := engineMove.AlgebraMove
	if move == "" {
		move, err = getAlgebraMoveFromChessGame(chessGame, engineMove.CoordinateMove)
	}
	if err != nil {
		log.WithError(err).Error("cannot convert engine coordinate move")
		return nil
	}
	move = normalizeEngineMove(move)

	// Preserve the engine's draw offer before applying its move.
	if engineMove.WillAcceptDraw {
		err = OfferDraw(game.ID, cmpName, game.TurnColor())
		if err != nil {
			if isRetryableMoveResponseError(err) {
				return err
			}
			log.WithError(err).Error("discarding invalid engine draw offer")
			return nil
		}
	}

	// Apply through the existing locked index, turn, and chess validation.
	moveData := models.MoveData2{
		Index: engineMove.Index,
		Move:  move,
	}
	err = AddMove(game.ID, cmpName, moveData)
	if err != nil {
		if isRetryableMoveResponseError(err) {
			return err
		}
		log.WithError(err).Error("discarding invalid engine move response")
	}
	return nil
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
