package models

type ScoreboardRow struct {
	Personality string         `json:"personality"`
	Games       int            `json:"games"`
	Wins        map[string]int `json:"wins"`
	Draws       int            `json:"draws"`
}

type Scoreboard struct {
	GameTag        string          `json:"gameTag"`
	WorkerTags     []string        `json:"workerTags"`
	Games          int             `json:"games"`
	CompletedPairs int             `json:"completedPairs"`
	Wins           map[string]int  `json:"wins"`
	Draws          int             `json:"draws"`
	Personalities  []ScoreboardRow `json:"personalities"`
}
