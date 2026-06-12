package lichess

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
)

var baseUrl = "https://lichess.org/api"
var token = os.Getenv("LICHESS_TOKEN")

type LichessInfo struct {
	ID    string `json:"id"`
	URL   string `json:"url"`
	IsNew bool   `json:"isNew"`
}

func ImportGame(pgn string) (LichessInfo, error) {
	body := strings.NewReader("pgn=" + url.QueryEscape(pgn))
	req, err := http.NewRequest("POST", baseUrl+"/import", body)
	if err != nil {
		return LichessInfo{}, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return LichessInfo{}, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return LichessInfo{}, fmt.Errorf("import game request failed with status %d", res.StatusCode)
	}

	var lichessInfo LichessInfo
	if err := json.NewDecoder(res.Body).Decode(&lichessInfo); err != nil {
		return LichessInfo{}, err
	}

	lichessInfo.IsNew = true

	return lichessInfo, nil
}
