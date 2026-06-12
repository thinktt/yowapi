package models

import "fmt"

type Player struct {
	ID   string `json:"id" binding:"required"` //needs lichess id validation
	Type string `json:"type" binding:"required,oneof=cmp lichess lichessBot"`
}
type Game2New struct {
	LichessID   string `json:"lichessId" bson:"lichessId" binding:"omitempty,alphanum,min=8,max=8"`
	WhitePlayer Player `json:"whitePlayer" bson:"whitePlayer" binding:"required"`
	BlackPlayer Player `json:"blackPlayer" bson:"blackPlayer" binding:"required"`
}

type Game2 struct {
	ID            string   `json:"id" bson:"id" binding:"required,alphanum,min=8,max=9"`
	LichessID     string   `json:"lichessId" bson:"lichessId"  binding:"alphanum,min=8,max=8"`
	CreatedAt     int64    `json:"createdAt" bson:"createdAt" binding:"required"`
	LastMoveAt    int64    `json:"lastMoveAt" bson:"lastMoveAt" binding:"required"`
	Winner        string   `json:"winner" bson:"winner" binding:"required,oneof=pending white black draw"`
	Method        string   `json:"method,omitempty" bson:"method,omitempty" binding:"omitempty,oneof=mate resign material mutual stalemate threefold fiftyMove time"`
	Moves         string   `json:"moves" bson:"moves,omitempty" binding:"required"`
	MoveList      []string `json:"moveList,omitempty" bson:"moveList"`
	WhitePlayer   Player   `json:"whitePlayer" bson:"whitePlayer" binding:"required"`
	BlackPlayer   Player   `json:"blackPlayer" bson:"blackPlayer" binding:"required"`
	WhiteWillDraw bool     `json:"whiteWillDraw,omitempty" bson:"whiteWillDraw"`
	BlackWillDraw bool     `json:"blackWillDraw,omitempty" bson:"blackWillDraw"`
	// Status      string   `json:"status" bson:"status" binding:"required,oneof=created started mate resign stalemate draw"`
	// DrawType    string   `json:"drawType,omitempty" bson:"drawType" binding:"omitempty,oneof=material stalemate threefold fiftyMove mutual"`
}

func (g *Game2) TurnColor() string {
	if len(g.MoveList)%2 == 0 {
		return "white"
	}
	return "black"
}
func (g *Game2) HasPlayer(userID string) bool {
	if g.WhitePlayer.ID == userID || g.BlackPlayer.ID == userID {
		return true
	}
	return false
}

func (g *Game2) UserIsColor(userID, color string) bool {
	if color == "white" && g.WhitePlayer.ID == userID {
		return true
	}
	if color == "black" && g.BlackPlayer.ID == userID {
		return true
	}
	return false
}

func (g *Game2) IsUsersTurn(userID string) bool {
	turnColor := g.TurnColor()
	if turnColor == "white" && g.WhitePlayer.ID == userID {
		return true
	}
	if turnColor == "black" && g.BlackPlayer.ID == userID {
		return true
	}
	return false
}

func (g *Game2) GetUsercolor(userID string) (string, error) {
	if userID == g.WhitePlayer.ID {
		return "white", nil
	}

	if userID == g.BlackPlayer.ID {
		return "black", nil
	}

	// this user is not playing this game
	return "", fmt.Errorf("user not found in game")
}

type Game2MutableFields struct {
	ID            string `json:"id" binding:"required,alphanum,min=8,max=8"`
	LichessID     string `json:"lichessId,omitempty" bson:"lichessId"  binding:"alphanum,min=8,max=8"`
	LastMoveAt    int64  `json:"lastMoveAt" binding:"required"`
	Moves         string `json:"moves" binding:"required"`
	Winner        string `json:"winner,omitempty" binding:"required,oneof=white black"`
	Method        string `json:"method,omitempty" binding:"requied,oneof=mate resign material mutual stalemate threefold fiftyMove"`
	WhiteWillDraw bool   `json:"whiteWillDraw,omitempty"`
	BlackWillDraw bool   `json:"blackWillDraw,omitempty"`
	// Status     string `json:"status,omitempty" binding:"required,oneof=started mate resign draw"`
}

// DrawType   string `json:"drawType" bson:"drawType,omitempty" binding:"required,oneof=mutual fifty stalemate material"`
// LichessID   string   `json:"lichessId" bson:"lichessId"  binding:"alphanum,min=8,max=8"`
// MoveList    []string `json:"moveList,omitempty" bson:"moveList"`

type MoveData2 struct {
	Index int    `json:"index"`
	Move  string `json:"move" binding:"required"`
}
