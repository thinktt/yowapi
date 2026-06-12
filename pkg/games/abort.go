package games

import (
	"fmt"
	"net/http"

	"github.com/thinktt/yowapi/pkg/db"
	"github.com/thinktt/yowapi/pkg/utils"
)

func Abort(id, userID, color string) error {
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

	// check if the game has more than two moves
	if len(game.MoveList) > 2 {
		return utils.NewHTTPError(http.StatusBadRequest, "cannot abort, too many moves")
	}

	// delete the game
	_, err = db.DeleteGame2(id)
	if err != nil {
		return utils.NewHTTPError(http.StatusInternalServerError, "DB Error: "+err.Error())
	}

	return nil
}
