package games

import (
	"fmt"
	"strings"

	"github.com/notnil/chess"
	"github.com/thinktt/yowapi/pkg/models"
)

func UserIsColor(game models.Game2, user, color string) bool {
	if color == "white" && game.WhitePlayer.ID == user {
		return true
	}
	if color == "black" && game.BlackPlayer.ID == user {
		return true
	}
	return false
}

// GetTurnColor given a move index returns the color that move belongs to
func GetTurnColor(moveIndex int) string {
	if moveIndex%2 == 0 {
		return "white"
	}
	return "black"
}

func GetUsercolor(game models.Game2, user string) string {
	if user == game.WhitePlayer.ID {
		return "white"
	}

	if user == game.BlackPlayer.ID {
		return "black"
	}

	// this user is not playing this game
	return ""
}

func GetOpponentColor(userColor string) string {
	if userColor == "white" {
		return "black"
	}
	if userColor == "black" {
		return "white"
	}
	return ""
}

func ParseGame(game models.Game2) (*chess.Game, error) {
	chessGame := chess.NewGame()
	for i, move := range game.MoveList {
		err := chessGame.MoveStr(move)
		if err != nil {
			err := fmt.Errorf("error parsing game at index %d: %v", i, err)
			return nil, err
		}
	}

	return chessGame, nil

}

func GetGameStatus(chessGame *chess.Game) (winner, method string) {

	chessGame.Draw(chess.FiftyMoveRule)
	chessGame.Draw(chess.ThreefoldRepetition)

	ending := chessGame.Method()

	// oneof=mate resign material mutual stalemate threefold fiftyMove
	winner = "pending"
	method = ""
	switch ending {
	case chess.Checkmate:
		method = "mate"
		if chessGame.Outcome() == chess.WhiteWon {
			winner = "white"
		} else {
			winner = "black"
		}
	case chess.Stalemate:
		winner = "draw"
		method = "stalemate"
	case chess.InsufficientMaterial:
		winner = "draw"
		method = "material"
	case chess.ThreefoldRepetition:
		winner = "draw"
		method = "threefold"
	case chess.FiftyMoveRule:
		winner = "draw"
		method = "fiftyMove"
	}

	return
}

func GetProperLastMove(chessGame *chess.Game) (string, error) {
	pgn := chessGame.String()
	pgnSlice := strings.Fields(pgn)
	if len(pgnSlice) < 3 {
		err := fmt.Errorf("unable parse move from chessGame: PGN too short")
		return "", err
	}

	move := pgnSlice[len(pgnSlice)-2]
	return move, nil
}
