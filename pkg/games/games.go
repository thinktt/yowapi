package games

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/akamensky/base58"
	"github.com/notnil/chess"
	"github.com/thinktt/yowapi/pkg/db"
	"github.com/thinktt/yowapi/pkg/events"
	"github.com/thinktt/yowapi/pkg/lichess"
	"github.com/thinktt/yowapi/pkg/models"
)

// PublishGameUPdates takes a game ID and gets that game from the DB and then
// derives the gameUpdate from the game and publishes the update to the streams
func PublishGameUpdates(gameID string) error {

	game, err := db.GetGame2(gameID)
	if err != nil {
		return err
	}

	gameUpdate := GetGameUpdate(game)

	jsonData, _ := json.Marshal(gameUpdate)
	events.Pub.PublishMessage(game.ID, string(jsonData))

	go PlayEngineMove(game)

	return nil
}

func GetGameUpdate(game models.Game2) models.Game2MutableFields {

	// if winner hasn't change omit winner and method from update
	winner := ""
	method := ""
	lichessID := ""

	if game.Winner != "pending" {
		winner = game.Winner
		method = game.Method
		lichessID = game.LichessID
	}

	gameMuation := models.Game2MutableFields{
		ID:            game.ID,
		LichessID:     lichessID,
		LastMoveAt:    game.LastMoveAt,
		Moves:         game.Moves,
		Winner:        winner,
		Method:        method,
		WhiteWillDraw: game.WhiteWillDraw,
		BlackWillDraw: game.BlackWillDraw,
	}

	return gameMuation
}

// var uciRegex = regexp.MustCompile(`[a-h][1-8][a-h][1-8][qrbn]?`)

func getAlgebraMoveFromChessGame(chessGame *chess.Game, newUciMove string) (string, error) {
	err := chessGame.MoveStr(newUciMove)
	if err != nil {
		err := fmt.Errorf("error adding coordinate move to chessGame: %s", err.Error())
		return "", err
	}

	chess.UseNotation(chess.AlgebraicNotation{})(chessGame)
	pgn := chessGame.String()
	pgnSlice := strings.Fields(pgn)
	if len(pgnSlice) < 3 {
		err := fmt.Errorf("unable parse algebra move from chessGame: PGN too short")
		return "", err
	}

	algebraMove := pgnSlice[len(pgnSlice)-2]
	return algebraMove, nil
}

func getUCIMovesFromChessGame(chessGame *chess.Game) ([]string, error) {
	chess.UseNotation(chess.UCINotation{})(chessGame)
	moves := make([]string, 0, len(chessGame.Moves()))

	for _, move := range chessGame.Moves() {
		moves = append(moves, move.String())
	}

	return moves, nil
}

func parseToChessGame(moves []string) (*chess.Game, error) {
	chessGame := chess.NewGame()
	for i, move := range moves {
		err := chessGame.MoveStr(move)
		if err != nil {
			errMsg := fmt.Sprintf("Invalid move at index %d: %v", i, err)
			return nil, errors.New(errMsg)
		}
	}
	return chessGame, nil
}

func GetAlgebraicNotation(uciMoves []string) ([]string, error) {
	gameParser := chess.NewGame(chess.UseNotation(chess.UCINotation{}))
	algebraicMoves := make([]string, 0, len(uciMoves))

	for i, move := range uciMoves {
		err := gameParser.MoveStr(move)
		if err != nil {
			errMsg := fmt.Sprintf("Invalid UCI move at index %d: %v", i, err)
			return nil, errors.New(errMsg)
		}
		algebraicMove := gameParser.Moves()[i].String()
		algebraicMoves = append(algebraicMoves, algebraicMove)
	}

	return algebraicMoves, nil
}

func GetGameID() (string, error) {
	// Create a byte array of length 6
	b := make([]byte, 6)

	// Fill the byte array with random bytes
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	// Encode the byte array to a Base58 string
	encoded := base58.Encode(b)

	// If the encoded length is 9, chop off the first letter
	if len(encoded) == 9 {
		encoded = encoded[1:]
	}

	// add a y on the end to distinguish from lichess id
	id := encoded + "y"

	return id, nil
}

func GetPGN(gameID string) (string, error) {
	game, err := db.GetGame2(gameID)
	if err != nil {
		return "", err
	}

	pgn, err := buildPGN(game, true)
	if err != nil {
		return "", err
	}

	return pgn, nil
}

func buildPGN(game models.Game2, isForLichess bool) (string, error) {
	pgnTemplate := `
	[Event "Ye Old Wizard Game"]
	[Site "https://yeoldwizard.com/#board/%s"]
	[White "%s"]
	[Black "%s"]
	[UTCDate "%s"]
	[UTCTime "%s"]
`

	pgn := fmt.Sprintf(pgnTemplate,
		game.ID,
		game.WhitePlayer.ID,
		game.BlackPlayer.ID,
		time.Unix(game.CreatedAt/1000, 0).Format("2006.01.02"),
		time.Unix(game.CreatedAt/1000, 0).Format("15:04:05"),
	)

	pgn = stripIndentation(pgn)

	// adding this gets lichess to recognize end score resignation
	if game.Method == "resign" {
		pgn = pgn + "\n" + `[Termination "Normal"]`
	}

	pgnMoves, err := buildPGNMoves(game.MoveList)
	if err != nil {
		return "", err
	}

	resultMap := map[string]string{
		"pending": "*", "white": "1-0", "black": "0-1", "draw": "1/2-1/2"}
	result := resultMap[game.Winner]

	if isForLichess {
		result = getLichessResult(game, result)
	}

	pgn = pgn + "\n\n" + pgnMoves + result

	return pgn, nil
}

// getLichessResults is to deal with strange issues in the lichess pgn import
// algorithm parsing results, it tries to sanatize as best as possible reults
// so they turn out better when imported
func getLichessResult(game models.Game2, result string) string {

	// leave result off if lichess can parse the resullt from moves alone
	isBoardParsable := map[string]bool{
		"stalemate": true, "mate": true, "material": true, "fiftyMove": true,
	}
	if isBoardParsable[game.Method] {
		result = ""
	}

	return result
}

func buildPGNMoves(moves []string) (string, error) {
	if len(moves) == 0 {
		return "", fmt.Errorf("no moves provided")
	}

	var pgnMoves strings.Builder
	for i, move := range moves {
		if i%2 == 0 {
			pgnMoves.WriteString(fmt.Sprintf("%d. ", i/2+1))
		}
		pgnMoves.WriteString(move + " ")
	}
	return pgnMoves.String(), nil
}

func stripIndentation(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimLeft(line, "\t ")
	}
	return strings.Join(lines, "\n")
}

func GetLichessInfo(gameID string) (lichess.LichessInfo, error) {
	game, err := db.GetGame2(gameID)
	if err != nil {
		return lichess.LichessInfo{}, err
	}

	if game.ID == "" {
		return lichess.LichessInfo{}, fmt.Errorf("game not found for ID: %s", gameID)
	}

	// this game already has a lichess mirror
	if game.LichessID != "" {
		lichessInfo := lichess.LichessInfo{
			ID:    game.LichessID,
			URL:   "https://lichess.org/" + game.LichessID,
			IsNew: false,
		}
		return lichessInfo, nil
	}

	lichessInfo, err := CreateLichessGame(game)
	if err != nil {
		return lichess.LichessInfo{}, fmt.Errorf("error sending pgn to lichess: %w", err)
	}

	return lichessInfo, nil
}

func CreateLichessGame(game models.Game2) (lichess.LichessInfo, error) {

	pgn, err := buildPGN(game, true)
	fmt.Println(game)
	if err != nil {
		return lichess.LichessInfo{}, fmt.Errorf("error building pgn: %w", err)
	}

	// send game to lichess
	lichessInfo, err := lichess.ImportGame(pgn)
	if err != nil {
		return lichess.LichessInfo{}, fmt.Errorf("error sending pgn to lichess: %w", err)
	}

	_, err = db.UpdateLichessID(game.ID, lichessInfo.ID)
	if err != nil {
		return lichess.LichessInfo{}, err
	}

	return lichessInfo, nil
}
