package model

import (
	"database/sql"
	"encoding/json"
	"fmt"

	_ "modernc.org/sqlite"
)

const vcConfigSingletonKey = "default"

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	store := &SQLiteStore{db: db}
	if err := store.initSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteStore) initSchema() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS players (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			main_role TEXT NOT NULL DEFAULT '',
			highest_rank TEXT NOT NULL DEFAULT '',
			highest_division TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS vc_config (
			singleton_key TEXT PRIMARY KEY,
			category_id TEXT NOT NULL DEFAULT '',
			vc_channel_ids_json TEXT NOT NULL DEFAULT '[]'
		)`,
	}

	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("init schema: %w", err)
		}
	}
	return nil
}

func (s *SQLiteStore) UpsertPlayer(player PlayerInfo) error {
	_, err := s.db.Exec(`
		INSERT INTO players (id, name, main_role, highest_rank, highest_division)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			main_role = excluded.main_role,
			highest_rank = excluded.highest_rank,
			highest_division = excluded.highest_division
	`,
		player.ID,
		player.Name,
		player.MainRole,
		player.HighestRank.Rank,
		player.HighestRank.Division,
	)
	if err != nil {
		return fmt.Errorf("upsert player %s: %w", player.ID, err)
	}
	return nil
}

func (s *SQLiteStore) GetPlayerByID(id string) (*PlayerInfo, error) {
	var p PlayerInfo
	var rank string
	var division string
	err := s.db.QueryRow(`
		SELECT id, name, main_role, highest_rank, highest_division
		FROM players
		WHERE id = ?
	`, id).Scan(&p.ID, &p.Name, &p.MainRole, &rank, &division)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get player %s: %w", id, err)
	}
	p.HighestRank = Rank{Rank: rank, Division: division}
	return &p, nil
}

func (s *SQLiteStore) PlayerCount() (int, error) {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM players`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count players: %w", err)
	}
	return n, nil
}

func (s *SQLiteStore) LoadVCConfig() (VCConfig, error) {
	var cfg VCConfig
	var idsJSON string
	err := s.db.QueryRow(`
		SELECT category_id, vc_channel_ids_json
		FROM vc_config
		WHERE singleton_key = ?
	`, vcConfigSingletonKey).Scan(&cfg.CategoryID, &idsJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			cfg.VCChannelIDs = []string{}
			return cfg, nil
		}
		return cfg, fmt.Errorf("load vc config: %w", err)
	}
	if err := json.Unmarshal([]byte(idsJSON), &cfg.VCChannelIDs); err != nil {
		return VCConfig{}, fmt.Errorf("decode vc channel ids: %w", err)
	}
	if cfg.VCChannelIDs == nil {
		cfg.VCChannelIDs = []string{}
	}
	return cfg, nil
}

func (s *SQLiteStore) SaveVCConfig(cfg VCConfig) error {
	if cfg.VCChannelIDs == nil {
		cfg.VCChannelIDs = []string{}
	}
	body, err := json.Marshal(cfg.VCChannelIDs)
	if err != nil {
		return fmt.Errorf("encode vc channel ids: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO vc_config (singleton_key, category_id, vc_channel_ids_json)
		VALUES (?, ?, ?)
		ON CONFLICT(singleton_key) DO UPDATE SET
			category_id = excluded.category_id,
			vc_channel_ids_json = excluded.vc_channel_ids_json
	`, vcConfigSingletonKey, cfg.CategoryID, string(body))
	if err != nil {
		return fmt.Errorf("save vc config: %w", err)
	}
	return nil
}
