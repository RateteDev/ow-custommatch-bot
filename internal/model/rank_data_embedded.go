package model

import (
	_ "embed"
	"encoding/json"
)

//go:embed rankdata/rank.json
var embeddedRankJSON []byte

func LoadEmbeddedRankData() (RankDataFile, error) {
	var r RankDataFile
	if err := json.Unmarshal(embeddedRankJSON, &r); err != nil {
		return r, err
	}
	return r, nil
}
