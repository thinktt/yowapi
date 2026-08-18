package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/thinktt/yowapi/pkg/db"
	"github.com/thinktt/yowapi/pkg/engine"
	"github.com/thinktt/yowapi/pkg/events"
	"github.com/thinktt/yowapi/pkg/games"
	"github.com/thinktt/yowapi/pkg/models"
	"github.com/thinktt/yowapi/pkg/moveque"
	"github.com/thinktt/yowapi/pkg/utils"
)

// requestEngineMove continues a game when the current player is an engine.
func requestEngineMove(gameID string) error {
	game, err := db.GetGame2(gameID)
	if err != nil {
		return fmt.Errorf("load game for engine move: %w", err)
	}
	if game.ID == "" {
		return fmt.Errorf("game %s not found", gameID)
	}

	moveReq, shouldMove, err := engine.BuildMoveRequest(game)
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

// continueGame starts the next engine turn and logs failures outside the completed action.
func continueGame(gameID string) {
	err := requestEngineMove(gameID)
	if err != nil {
		logrus.WithField("gameId", gameID).WithError(err).Error("unable to continue game")
	}
}

// handleEngineResponse composes a worker response with the normal game move flow.
func handleEngineResponse(response models.MoveData) error {
	log := logrus.WithFields(logrus.Fields{
		"gameId":    response.GameId,
		"index":     response.Index,
		"workerTag": response.WorkerTag,
	})

	if response.GameId == "" {
		log.Error("discarding move response with no game ID")
		return nil
	}

	publishEngineResponseEvents(response, log)
	if response.Err != nil {
		log.WithField("engineError", *response.Err).Error("discarding worker move error")
		return nil
	}

	game, err := db.GetGame2(response.GameId)
	if err != nil {
		return fmt.Errorf("load game for move response: %w", err)
	}
	if game.ID == "" {
		log.Warn("move response references a missing game")
		return nil
	}

	engineMove, err := engine.ParseMoveResponse(game, response)
	if err != nil {
		log.WithError(err).Error("discarding invalid engine move response")
		return nil
	}

	if engineMove.WillAcceptDraw {
		err = games.OfferDraw(engineMove.GameID, engineMove.PlayerID, engineMove.PlayerColor)
		if err != nil {
			return handleMoveResponseError(err, log, "discarding invalid engine draw offer")
		}
	}

	moveData := models.MoveData2{
		Index: engineMove.Index,
		Move:  engineMove.Move,
	}
	err = games.AddMove(engineMove.GameID, engineMove.PlayerID, moveData)
	if err != nil {
		return handleMoveResponseError(err, log, "discarding invalid engine move response")
	}

	go continueGame(engineMove.GameID)
	return nil
}

func handleMoveResponseError(err error, log *logrus.Entry, message string) error {
	if isRetryableMoveResponseError(err) {
		return err
	}
	log.WithError(err).Error(message)
	return nil
}

func publishEngineResponseEvents(response models.MoveData, log *logrus.Entry) {
	if response.Err != nil {
		publishEngineResponseEvent("engineError", response, *response.Err)
		return
	}
	if response.Warning == nil {
		return
	}

	publishEngineResponseEvent("engineWarning", response, *response.Warning)
	log.WithFields(logrus.Fields{
		"engineWarning":  *response.Warning,
		"algebraMove":    response.AlgebraMove,
		"coordinateMove": response.CoordinateMove,
	}).Warn("worker returned a move warning")
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

// NATS retries infrastructure failures but consumes invalid game responses.
func isRetryableMoveResponseError(err error) bool {
	var httpErr *utils.HTTPError
	if !errors.As(err, &httpErr) {
		return true
	}
	return httpErr.StatusCode == http.StatusInternalServerError &&
		strings.HasPrefix(httpErr.Message, "DB Error:")
}
