package model

import (
	"encoding/json"
	"errors"
	"os"
)

type VCConfig struct {
	CategoryID   string   `json:"category_id"`
	VCChannelIDs []string `json:"vc_channel_ids"`
}

type VCConfigManager struct {
	path string
	Data VCConfig
}

func NewVCConfigManager(path string) *VCConfigManager {
	return &VCConfigManager{path: path}
}

func (m *VCConfigManager) Load() error {
	if _, err := os.Stat(m.path); errors.Is(err, os.ErrNotExist) {
		m.Data = VCConfig{VCChannelIDs: []string{}}
		return nil
	}

	body, err := os.ReadFile(m.path)
	if err != nil {
		return err
	}
	if len(body) == 0 {
		m.Data = VCConfig{VCChannelIDs: []string{}}
		return nil
	}
	if err := json.Unmarshal(body, &m.Data); err != nil {
		return err
	}
	if m.Data.VCChannelIDs == nil {
		m.Data.VCChannelIDs = []string{}
	}
	return nil
}

func (m *VCConfigManager) Save() error {
	body, err := json.MarshalIndent(m.Data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.path, body, 0o644)
}
