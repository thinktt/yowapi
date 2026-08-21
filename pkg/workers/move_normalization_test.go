package workers

import (
	"strings"
	"testing"

	"github.com/thinktt/yowapi/pkg/games"
	"github.com/thinktt/yowapi/pkg/models"
)

func TestNormalizeEngineMoveCheckmateSuffix(t *testing.T) {
	// Game jhSMidHyy reached a promotion mate after a yowking worker reported
	// Qg8+. The chess library correctly rejects that notation because Qg8 is
	// checkmate, not merely check. yowapi owns the game state, so it removes
	// the engine's check marker and lets the chess library record Qg8#.
	moves := strings.Fields(`
		d4 d5 Nf3 Nf6 e3 c6 Ne5 Qc7 Bd3 e6 a3 Qa5+ Bd2 Qc7 Bc1 Bd6 Bd2 Bxe5
		dxe5 Nfd7 Kf1 Nxe5 Bc3 Nbd7 a4 Nf6 h3 Bd7 Ra2 Nxd3 cxd3 e5 Na3 d4 exd4
		Be6 dxe5 Nd5 Bd4 Qe7 b3 c5 Ba1 O-O Nc4 Nb4 Re2 Rfd8 Nd6 Nxd3 Qxd3 a5
		Qb5 Rxd6 exd6 Qxd6 Re1 Qd2 f3 Qd5 Qe2 Rc8 Qe3 Re8 Kg1 f6 Qc3 Ra8 f4
		Bf7 Kh2 Qc6 Rd1 Kh8 Rhe1 Bh5 Rd2 Rg8 Rc1 b6 Rf1 Qc7 Qd3 Qe7 Rc1 Re8
		Bc3 c4 bxc4 Qc5 Qg3 Qc6 Bxa5 bxa5 Qd3 Qxa4 Qd5 Bg6 Qd6 Qb3 c5 h5 Rf1
		Qc4 Rff2 Bf7 c6 Rc8 Qe7 Bd5 c7 Kh7 Qd7 Be6 Qd6 Bf5 Qd5 Qxd5 Rxd5 Kg6
		Rc5 a4 Rf3 Kf7 Ra3 g6 Rxa4 h4 Rd4 Ke7 Rd2 Be4 Kg1 Bf5 Rdd5 Be6 Rd4 f5
		Rc3 Ra8 Rc1 Rc8 Rd3 Kf8 Rc6 Kg7 Rdd6 Bf7 Rd8 Be6 Kf2 Kf7 Ke3 Kg7 Kd4
		g5 Ke5 Kf7 fxg5 f4 g6+ Kg7 Kxe6 f3 Rxc8 f2 Rg8+ Kh6 Rh8+ Kxg6 Ke5+
		Kg7 c8=Q f1=Q
	`)

	rawGame, err := games.ParseGame(models.Game2{MoveList: strings.Fields(moves)})
	if err != nil {
		t.Fatalf("parseToChessGame() error = %v", err)
	}
	if err := rawGame.MoveStr("Qg8+"); err == nil {
		t.Fatal("chess library accepted Qg8+ when the move is checkmate")
	}

	chessGame, err := games.ParseGame(models.Game2{MoveList: strings.Fields(moves)})
	if err != nil {
		t.Fatalf("parseToChessGame() error = %v", err)
	}

	move := normalizeEngineMove("Qg8+")
	if move != "Qg8" {
		t.Errorf("normalizeEngineMove() = %q, want %q", move, "Qg8")
	}
	if err := chessGame.MoveStr(move); err != nil {
		t.Fatalf("chess library rejected normalized move %q: %v", move, err)
	}

	properMove, err := games.GetProperLastMove(chessGame)
	if err != nil {
		t.Fatalf("GetProperLastMove() error = %v", err)
	}
	if properMove != "Qg8#" {
		t.Errorf("GetProperLastMove() = %q, want %q", properMove, "Qg8#")
	}

	winner, method := games.GetGameStatus(chessGame)
	if winner != "white" || method != "mate" {
		t.Errorf("GetGameStatus() = (%q, %q), want (%q, %q)", winner, method, "white", "mate")
	}
}
