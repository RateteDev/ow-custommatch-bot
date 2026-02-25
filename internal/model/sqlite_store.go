package model

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

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
			highest_division TEXT NOT NULL DEFAULT '',
			rank_updated_at TEXT NOT NULL DEFAULT ''
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
	if _, err := s.db.Exec(`ALTER TABLE players ADD COLUMN rank_updated_at TEXT NOT NULL DEFAULT ''`); err != nil {
		// 既存DBでは既に追加済みの可能性があるため duplicate column は許容する。
		if !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			return fmt.Errorf("init schema alter players.rank_updated_at: %w", err)
		}
	}
	return nil
}

func (s *SQLiteStore) UpsertPlayer(player PlayerInfo) error {
	_, err := s.db.Exec(`
		INSERT INTO players (id, name, main_role, highest_rank, highest_division, rank_updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			main_role = excluded.main_role,
			highest_rank = excluded.highest_rank,
			highest_division = excluded.highest_division,
			rank_updated_at = excluded.rank_updated_at
	`,
		player.ID,
		player.Name,
		player.MainRole,
		player.HighestRank.Rank,
		player.HighestRank.Division,
		player.RankUpdatedAt,
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
	var rankUpdatedAt string
	err := s.db.QueryRow(`
		SELECT id, name, main_role, highest_rank, highest_division, rank_updated_at
		FROM players
		WHERE id = ?
	`, id).Scan(&p.ID, &p.Name, &p.MainRole, &rank, &division, &rankUpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get player %s: %w", id, err)
	}
	p.HighestRank = Rank{Rank: rank, Division: division}
	p.RankUpdatedAt = rankUpdatedAt
	return &p, nil
}

func (s *SQLiteStore) PlayerCount() (int, error) {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM players`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count players: %w", err)
	}
	return n, nil
}

func (s *SQLiteStore) ListPlayers() ([]PlayerInfo, error) {
	rows, err := s.db.Query(`
		SELECT id, name, main_role, highest_rank, highest_division, rank_updated_at
		FROM players
		ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("list players: %w", err)
	}
	defer rows.Close()

	players := []PlayerInfo{}
	for rows.Next() {
		var p PlayerInfo
		var rank string
		var division string
		var rankUpdatedAt string
		if err := rows.Scan(&p.ID, &p.Name, &p.MainRole, &rank, &division, &rankUpdatedAt); err != nil {
			return nil, fmt.Errorf("scan player row: %w", err)
		}
		p.HighestRank = Rank{Rank: rank, Division: division}
		p.RankUpdatedAt = rankUpdatedAt
		players = append(players, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate players: %w", err)
	}
	return players, nil
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

func (s *SQLiteStore) HasVCConfig() (bool, error) {
	var n int
	if err := s.db.QueryRow(`
		SELECT COUNT(*)
		FROM vc_config
		WHERE singleton_key = ?
	`, vcConfigSingletonKey).Scan(&n); err != nil {
		return false, fmt.Errorf("check vc config row: %w", err)
	}
	return n > 0, nil
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
