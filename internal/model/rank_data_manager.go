package model

import (
	"encoding/json"
	"os"
)

type Rank struct {
	Rank     string `json:"rank"`
	Division string `json:"division,omitempty"`
}

type RankTable map[string]map[string]float64

type RankDataFile struct {
	Ranks RankTable `json:"ranks"`
}

func LoadRankData(path string) (RankDataFile, error) {
	var r RankDataFile
	body, err := os.ReadFile(path)
	if err != nil {
		return r, err
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return r, err
	}
	return r, nil
}
