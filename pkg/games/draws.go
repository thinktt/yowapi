package games

import (
	"fmt"
	"net/http"

	"github.com/thinktt/yowapi/pkg/db"
	"github.com/thinktt/yowapi/pkg/utils"
)

// OfferDraw NEED TO CHECK THIS. Things could go wrong if cmp is playing both sides
func OfferDraw(id, userID, color string) error {
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

	// check if user is proper color
	if !game.UserIsColor(userID, color) {
		return utils.NewHTTPError(http.StatusBadRequest,
			fmt.Sprintf("you are not playing as %s", color))
	}

	// check if the other player has a draw offer
	opponentColor := GetOpponentColor(color)
	opponentWillDraw := false
	if opponentColor == "white" {
		opponentWillDraw = game.WhiteWillDraw
	} else if opponentColor == "black" {
		opponentWillDraw = game.BlackWillDraw
	}

	if opponentWillDraw {
		// draw the game
		_, err = db.DrawGame(id, "mutual")
		if err != nil {
			return utils.NewHTTPError(http.StatusInternalServerError, "DB Error: "+err.Error())
		}

		// hack for now, since game is finished mirror it on lichess and update the
		// lichessID. lichess stuff should be moved to a lichess bot service later
		// this is currently in the move, draw and resign func
		game, err = db.GetGame2(id)
		if err != nil {
			fmt.Printf("error mirroring %s to lichess %s", game.ID, err.Error())
		}
		_, err = CreateLichessGame(game)
		if err != nil {
			fmt.Printf("error mirroring %s to lichess %s", game.ID, err.Error())
		}

		PublishGameUpdates(game.ID)

		return nil
	}

	// update will draw state
	_, err = db.UpdateWillDraw(id, color, true)
	if err != nil {
		return utils.NewHTTPError(http.StatusInternalServerError, "DB Error: "+err.Error())
	}

	return nil
}

func ClearDrawOffer(id, userID, color string) error {
	// get the current game from the DB
	game, err := db.GetGame2(id)
	if err != nil {
		return utils.NewHTTPError(http.StatusInternalServerError, "DB Error: "+err.Error())
	}
	if game.ID == "" {
		return utils.NewHTTPError(http.StatusNotFound, fmt.Sprintf("no game found for id %s", id))
	}

	// check that game is still live
	if game.Winner != "pending" {
		return utils.NewHTTPError(http.StatusBadRequest, "cannot clear draw offer, game is finished")
	}
	// check if user is playing this game
	if !game.HasPlayer(userID) {
		return utils.NewHTTPError(http.StatusBadRequest, "not your game")
	}

	// check if user is proper color
	if !game.UserIsColor(userID, color) {
		return utils.NewHTTPError(http.StatusBadRequest,
			fmt.Sprintf("you are not playing as %s", color))
	}

	// clear the draw offer
	_, err = db.UpdateWillDraw(id, color, false)
	if err != nil {
		return utils.NewHTTPError(http.StatusInternalServerError, "DB Error: "+err.Error())
	}

	return nil
}
