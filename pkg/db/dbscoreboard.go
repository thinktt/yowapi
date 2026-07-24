package db

import (
	"context"
	"fmt"

	"github.com/thinktt/yowapi/pkg/models"
	"go.mongodb.org/mongo-driver/bson"
)

type scoreboardRun struct {
	ID            string   `bson:"_id"`
	GameTag       string   `bson:"gameTag"`
	WorkerTags    []string `bson:"workerTags"`
	BeeTag        string   `bson:"beeTag"`
	RyzTag        string   `bson:"ryzTag"`
	Personalities []string `bson:"personalities"`
}

type scoreboardJob struct {
	Personality string `bson:"personality"`
	Status      string `bson:"status"`
	Directions  map[string]struct {
		GameID string `bson:"gameId"`
	} `bson:"directions"`
}

type scoreboardGame struct {
	ID          string        `bson:"id"`
	Winner      string        `bson:"winner"`
	WhitePlayer models.Player `bson:"whitePlayer"`
	BlackPlayer models.Player `bson:"blackPlayer"`
}

// GetScoreboard returns completed mirrored-pair scores for a test game tag.
// Worker tags come from the run metadata and are used as response keys.
func GetScoreboard(gameTag string) (models.Scoreboard, error) {
	ctx := context.Background()
	runCollection := yowDatabase.Collection("arenaTestRuns")
	var run scoreboardRun
	if err := runCollection.FindOne(ctx, bson.M{"gameTag": gameTag}).Decode(&run); err != nil {
		return models.Scoreboard{}, fmt.Errorf("find scoreboard run: %w", err)
	}

	workerTags := run.WorkerTags
	if len(workerTags) == 0 {
		workerTags = []string{run.BeeTag, run.RyzTag}
	}
	if len(workerTags) != 2 || workerTags[0] == "" || workerTags[1] == "" || workerTags[0] == workerTags[1] {
		return models.Scoreboard{}, fmt.Errorf("scoreboard run must contain two distinct worker tags")
	}

	jobCursor, err := yowDatabase.Collection("arenaTestJobs").Find(ctx, bson.M{
		"runId":  run.ID,
		"status": bson.M{"$in": []string{"matched", "diverged"}},
	})
	if err != nil {
		return models.Scoreboard{}, fmt.Errorf("find scoreboard jobs: %w", err)
	}
	var jobs []scoreboardJob
	if err := jobCursor.All(ctx, &jobs); err != nil {
		return models.Scoreboard{}, fmt.Errorf("decode scoreboard jobs: %w", err)
	}

	gameIDs := make([]string, 0, len(jobs)*2)
	for _, job := range jobs {
		for _, direction := range job.Directions {
			if direction.GameID != "" {
				gameIDs = append(gameIDs, direction.GameID)
			}
		}
	}
	gameCursor, err := yowDatabase.Collection("games2").Find(ctx, bson.M{"id": bson.M{"$in": gameIDs}})
	if err != nil {
		return models.Scoreboard{}, fmt.Errorf("find scoreboard games: %w", err)
	}
	var gameDocuments []scoreboardGame
	if err := gameCursor.All(ctx, &gameDocuments); err != nil {
		return models.Scoreboard{}, fmt.Errorf("decode scoreboard games: %w", err)
	}
	games := make(map[string]scoreboardGame, len(gameDocuments))
	for _, game := range gameDocuments {
		games[game.ID] = game
	}

	result := models.Scoreboard{
		GameTag:       gameTag,
		WorkerTags:    workerTags,
		Wins:          map[string]int{workerTags[0]: 0, workerTags[1]: 0},
		Personalities: make([]models.ScoreboardRow, 0, len(run.Personalities)),
	}
	rows := make(map[string]*models.ScoreboardRow, len(run.Personalities))
	for _, personality := range run.Personalities {
		row := &models.ScoreboardRow{Personality: personality, Wins: map[string]int{workerTags[0]: 0, workerTags[1]: 0}}
		result.Personalities = append(result.Personalities, *row)
		rows[personality] = &result.Personalities[len(result.Personalities)-1]
	}

	for _, job := range jobs {
		if len(job.Directions) != 2 {
			continue
		}
		pairGames := make([]scoreboardGame, 0, 2)
		for _, direction := range job.Directions {
			game, ok := games[direction.GameID]
			if !ok || game.Winner == "pending" {
				pairGames = nil
				break
			}
			pairGames = append(pairGames, game)
		}
		if len(pairGames) != 2 {
			continue
		}

		result.CompletedPairs++
		row := rows[job.Personality]
		for _, game := range pairGames {
			result.Games++
			if game.Winner == "draw" {
				result.Draws++
				if row != nil {
					row.Draws++
				}
				continue
			}
			winnerTag := ""
			if game.Winner == "white" {
				winnerTag = game.WhitePlayer.WorkerTag
			} else if game.Winner == "black" {
				winnerTag = game.BlackPlayer.WorkerTag
			}
			if winnerTag != workerTags[0] && winnerTag != workerTags[1] {
				return models.Scoreboard{}, fmt.Errorf("game %s has unknown winner worker tag %q", game.ID, winnerTag)
			}
			result.Wins[winnerTag]++
			if row != nil {
				row.Wins[winnerTag]++
			}
		}
		if row != nil {
			row.Games += 2
		}
	}

	return result, nil
}
