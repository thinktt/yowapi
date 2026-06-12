package games

import (
	"fmt"
	"strings"

	"github.com/thinktt/yowapi/pkg/models"
	"github.com/thinktt/yowapi/pkg/moveque"
)

func PlayEngineMove(game models.Game2) {

	// the game is over, get out of here
	if game.Winner != "pending" {
		return
	}

	// look for any cmp playing this game
	var cmpName string
	if game.TurnColor() == "white" && game.WhitePlayer.Type == "cmp" {
		cmpName = game.WhitePlayer.ID
	}
	if game.TurnColor() == "black" && game.BlackPlayer.Type == "cmp" {
		cmpName = game.BlackPlayer.ID
	}

	// no cmp found for this turn, nothing needs to be done
	if cmpName == "" {
		// fmt.Println("not a cmp turn")
		return
	}

	chessGame, err := ParseGame(game)
	if err != nil {
		fmt.Println("Error parsing game: ", err.Error())
	}

	uciMoves, err := getUCIMovesFromChessGame(chessGame)
	if err != nil {
		fmt.Println("Error parsing UCI moves: ", err.Error())
	}

	moveReq := models.MoveReq{
		Moves:   uciMoves,
		CmpName: cmpName,
		GameId:  game.ID,
	}

	// get the next move from the engine workers
	engineMove, err := moveque.GetMove(moveReq)
	if err != nil {
		fmt.Println("Error getting engine move: ", err.Error())
		return
	}

	// have cmp set willDraw if the engine is willing to accept one
	if engineMove.WillAcceptDraw {
		// fmt.Println(game.TurnColor(), "is offering a draw")
		err = OfferDraw(game.ID, cmpName, game.TurnColor())
	}

	// if there is not an Algebra move we will need to translate the coordinate move
	move := engineMove.AlgebraMove
	if move == "" {
		move, err = getAlgebraMoveFromChessGame(chessGame, engineMove.CoordinateMove)
	}
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	move = normalizeEngineMove(move)

	moveData := models.MoveData2{
		Index: len(game.MoveList),
		Move:  move,
	}

	err = AddMove(game.ID, cmpName, moveData)
	if err != nil {
		fmt.Println("error Adding engine move: ", err.Error())
	}
}

func normalizeEngineMove(move string) string {

	// fix weird casling notation
	if strings.Contains(move, "0-0-0") {
		return "O-O-O"
	} else if strings.Contains(move, "0-0") {
		return "O-O"
	}

	// add = sign to promition moves
	for i := 1; i < len(move); i++ {
		if strings.ContainsRune("QNRB", rune(move[i])) {
			return move[:i] + "=" + move[i:]
		}
	}
	return move
}
