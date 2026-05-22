package config

import (
	"encoding/json"
	"fmt"
)

type v0Config struct {
	Language string    `json:"language,omitempty"`
	Profile  Profile   `json:"profile"`
	Projects []Project `json:"projects,omitempty"`
	Settings *Settings `json:"settings,omitempty"`
}

func migrateV0toV1(in []byte) (out []byte, newVersion int, err error) {
	version, err := schemaVersionOf(in)
	if err != nil {
		return nil, 0, err
	}
	if version >= Current {
		return in, version, nil
	}

	var legacy v0Config
	if err := json.Unmarshal(in, &legacy); err != nil {
		return nil, 0, fmt.Errorf("%w: %w", ErrInvalidJSON, err)
	}
	if legacy.Language == "" {
		legacy.Language = "en"
	}

	cfg := Config{
		SchemaVersion: Current,
		Language:      legacy.Language,
		Profiles:      []Profile{legacy.Profile},
		Projects:      legacy.Projects,
		Settings:      legacy.Settings,
	}
	if cfg.Projects == nil {
		cfg.Projects = []Project{}
	}

	out, err = json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, 0, fmt.Errorf("marshal migrated v0 config: %w", err)
	}
	out = append(out, '\n')
	return out, Current, nil
}
