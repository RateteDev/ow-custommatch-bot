package model

import (
	"encoding/json"
	"errors"
	"os"
)

type PlayerInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	MainRole    string `json:"main_role,omitempty"`
	HighestRank Rank   `json:"highest_rank,omitempty"`
}

type PlayersFile struct {
	Players []PlayerInfo `json:"players"`
}

type PlayerDataManager struct {
	filePath string
	sqlite   *SQLiteStore
	Data     PlayersFile
}

func NewPlayerDataManager(filePath string) (*PlayerDataManager, error) {
	mgr := &PlayerDataManager{filePath: filePath}
	if err := mgr.Load(); err != nil {
		return nil, err
	}
	return mgr, nil
}

func NewSQLitePlayerDataManager(store *SQLiteStore) (*PlayerDataManager, error) {
	mgr := &PlayerDataManager{sqlite: store}
	if err := mgr.Load(); err != nil {
		return nil, err
	}
	return mgr, nil
}

func (m *PlayerDataManager) Load() error {
	if m.sqlite != nil {
		players, err := m.sqlite.ListPlayers()
		if err != nil {
			return err
		}
		m.Data = PlayersFile{Players: players}
		return nil
	}

	if _, err := os.Stat(m.filePath); errors.Is(err, os.ErrNotExist) {
		m.Data = PlayersFile{Players: []PlayerInfo{}}
		return m.Save()
	}

	f, err := os.ReadFile(m.filePath)
	if err != nil {
		return err
	}

	return json.Unmarshal(f, &m.Data)
}

func (m *PlayerDataManager) Save() error {
	if m.sqlite != nil {
		for _, p := range m.Data.Players {
			if err := m.sqlite.UpsertPlayer(p); err != nil {
				return err
			}
		}
		return nil
	}

	body, err := json.MarshalIndent(m.Data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.filePath, body, 0o644)
}

func (m *PlayerDataManager) GetByID(id string) *PlayerInfo {
	for i := range m.Data.Players {
		if m.Data.Players[i].ID == id {
			return &m.Data.Players[i]
		}
	}
	return nil
}

func (m *PlayerDataManager) Add(player PlayerInfo) error {
	m.Data.Players = append(m.Data.Players, player)
	return m.Save()
}
