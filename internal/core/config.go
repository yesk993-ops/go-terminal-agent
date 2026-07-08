package core

import "time"

type Config struct {
	Provider  ProviderSettings `json:"provider"`
	Providers []ProviderConfig `json:"providers"`
	UI        UISettings       `json:"ui"`
	Session   SessionSettings  `json:"session"`
	Cache     CacheSettings    `json:"cache"`
	Logging   LogSettings      `json:"logging"`
}

type ProviderSettings struct {
	Default     string  `json:"default"`
	MaxTokens   int     `json:"max_tokens"`
	Temperature float64 `json:"temperature"`
}

type UISettings struct {
	Theme        string `json:"theme"`
	SyntaxTheme  string `json:"syntax_theme"`
	ShowTokens   bool   `json:"show_tokens"`
	ShowCost     bool   `json:"show_cost"`
	MaxHistoryUI int    `json:"max_history_ui"`
}

type SessionSettings struct {
	MaxMessages int           `json:"max_messages"`
	MaxAge      time.Duration `json:"max_age"`
	AutoSave    bool          `json:"auto_save"`
	SavePath    string        `json:"save_path"`
}

type CacheSettings struct {
	Enabled    bool          `json:"enabled"`
	MaxSize    int           `json:"max_size"`
	DefaultTTL time.Duration `json:"default_ttl"`
}

type LogSettings struct {
	Level  string `json:"level"`
	Format string `json:"format"`
	Output string `json:"output"`
}
