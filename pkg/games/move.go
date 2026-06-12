package games

import (
	"fmt"
	"net/http"

	"github.com/thinktt/yowapi/pkg/db"
	"github.com/thinktt/yowapi/pkg/models"
	"github.com/thinktt/yowapi/pkg/utils"
)

// AddMove validates and adds a move to the game, and then triggers an event
// message that moves were added, it returns cutsom HTTPErrors so errors can
// play nicely with an http routers
func AddMove(id string, userID string, moveData models.MoveData2) error {

	// get the current game from the DB
	game, err := db.GetGame2(id)
	if err != nil {
		err = utils.NewHTTPError(
			http.StatusInternalServerError, "DB Error: "+err.Error())
		return err
	}
	if game.ID == "" {
		err = utils.NewHTTPError(
			http.StatusNotFound,
			fmt.Sprintf("no game found for id %s", id))
		return err
	}

	//check that game is still live
	if game.Winner != "pending" {
		err = utils.NewHTTPError(
			http.StatusBadRequest, "no moves allowed, game is finished")
		return err
	}

	// check if user is playing this game
	if !game.HasPlayer(userID) {
		return utils.NewHTTPError(http.StatusBadRequest, "not your game")
	}

	// check if the move is at valid index
	if len(game.MoveList) != moveData.Index {
		err = utils.NewHTTPError(http.StatusBadRequest,
			fmt.Sprintf("invalid move index, next move index is %d", len(game.MoveList)))
		return err
	}

	// check if it is this user's turn
	if !game.IsUsersTurn(userID) {
		err = utils.NewHTTPError(http.StatusBadRequest, "not your turn")
		return err
	}

	// parse the db game into chessGame object
	chessGame, err := ParseGame(game)
	if err != nil {
		err = utils.NewHTTPError(
			http.StatusInternalServerError, "Error parsing db game for this move: "+err.Error())
		return err
	}

	// attempt to add the move to chessGame
	err = chessGame.MoveStr(moveData.Move)
	if err != nil {
		err = utils.NewHTTPError(http.StatusBadRequest,
			fmt.Sprintf("Ivalid move: %s", err.Error()))
		return err
	}

	// get proper last move, trusting chess lib, keeps db notation clean
	properMove, err := GetProperLastMove(chessGame)
	if err != nil {
		err = utils.NewHTTPError(http.StatusInternalServerError,
			fmt.Sprintf("Error parsing last move: %s", err.Error()))
		return err
	}

	// check if new move has changed the game status
	winner, method := GetGameStatus(chessGame)

	// choose db update method based on what needs to be updated
	if winner == "pending" {
		// game is in progress
		_, err = db.CreateMove(id, properMove, game.TurnColor())
	} else {
		// game is over
		_, err = db.UpdateGame(id, properMove, winner, method)

		// hack for now, since game is finished mirror it on lichess and update the
		// lichessID. lichess stuff should be moved to a lichess bot service later
		// this is currently in the move, draw and resign func
		game, err := db.GetGame2(id)
		if err != nil {
			fmt.Printf("error mirroring %s to lichess %s", game.ID, err.Error())
		}
		_, err = CreateLichessGame(game)
		if err != nil {
			fmt.Printf("error mirroring %s to lichess %s", game.ID, err.Error())
		}
	}
	if err != nil {
		err = utils.NewHTTPError(http.StatusInternalServerError, "DB Error: "+err.Error())
		return err
	}

	PublishGameUpdates(game.ID)

	return nil
}
