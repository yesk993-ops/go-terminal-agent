package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/agent/ai-terminal/internal/core"
	"github.com/spf13/viper"
)

const (
	appName    = "agent"
	configFile = "config.yaml"
)

func Load(path string) (*core.Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")

	v.SetDefault("provider.default", "openai")
	v.SetDefault("provider.max_tokens", 4096)
	v.SetDefault("provider.temperature", 0.7)

	v.SetDefault("ui.theme", "catppuccin-mocha")
	v.SetDefault("ui.syntax_theme", "catppuccin-mocha")
	v.SetDefault("ui.show_tokens", false)
	v.SetDefault("ui.show_cost", true)
	v.SetDefault("ui.max_history_ui", 50)

	v.SetDefault("session.max_messages", 100)
	v.SetDefault("session.max_age", "24h")
	v.SetDefault("session.auto_save", true)
	v.SetDefault("session.save_path", defaultSessionPath())

	v.SetDefault("cache.enabled", true)
	v.SetDefault("cache.max_size", 500)
	v.SetDefault("cache.default_ttl", "5m")

	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "text")
	v.SetDefault("logging.output", "stderr")

	v.SetEnvPrefix("AGENT")
	v.AutomaticEnv()

	if path != "" {
		if info, err := os.Stat(path); err == nil {
			if info.IsDir() {
				v.AddConfigPath(path)
			} else {
				v.SetConfigFile(path)
			}
		} else {
			v.AddConfigPath(path)
		}
	} else {
		xdgConfig := os.Getenv("XDG_CONFIG_HOME")
		if xdgConfig == "" {
			xdgConfig = filepath.Join(os.Getenv("HOME"), ".config")
		}
		searchPaths := []string{
			".",
			filepath.Join(os.Getenv("HOME"), ".config", appName),
			filepath.Join(xdgConfig, appName),
			"/etc/" + appName,
		}
		for _, p := range searchPaths {
			if p != "" {
				v.AddConfigPath(p)
			}
		}
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	providerConfigs := loadProviderConfigs(v)

	cfg := &core.Config{
		Provider: core.ProviderSettings{
			Default:     v.GetString("provider.default"),
			MaxTokens:   v.GetInt("provider.max_tokens"),
			Temperature: v.GetFloat64("provider.temperature"),
		},
		Providers: providerConfigs,
		UI: core.UISettings{
			Theme:        v.GetString("ui.theme"),
			SyntaxTheme:  v.GetString("ui.syntax_theme"),
			ShowTokens:   v.GetBool("ui.show_tokens"),
			ShowCost:     v.GetBool("ui.show_cost"),
			MaxHistoryUI: v.GetInt("ui.max_history_ui"),
		},
		Session: core.SessionSettings{
			MaxMessages: v.GetInt("session.max_messages"),
			MaxAge:      v.GetDuration("session.max_age"),
			AutoSave:    v.GetBool("session.auto_save"),
			SavePath:    v.GetString("session.save_path"),
		},
		Cache: core.CacheSettings{
			Enabled:    v.GetBool("cache.enabled"),
			MaxSize:    v.GetInt("cache.max_size"),
			DefaultTTL: v.GetDuration("cache.default_ttl"),
		},
		Logging: core.LogSettings{
			Level:  v.GetString("logging.level"),
			Format: v.GetString("logging.format"),
			Output: v.GetString("logging.output"),
		},
	}

	return cfg, nil
}

func loadProviderConfigs(v *viper.Viper) []core.ProviderConfig {
	envMappings := map[string]string{
		"openai":     "OPENAI_API_KEY",
		"anthropic":  "ANTHROPIC_API_KEY",
		"gemini":     "GEMINI_API_KEY",
		"groq":       "GROQ_API_KEY",
		"nvidia":     "NVIDIA_API_KEY",
		"openrouter": "OPENROUTER_API_KEY",
	}

	var configs []core.ProviderConfig

	for name, envKey := range envMappings {
		apiKey := os.Getenv(envKey)
		if apiKey == "" && v.IsSet(fmt.Sprintf("providers.%s.api_key", name)) {
			apiKey = v.GetString(fmt.Sprintf("providers.%s.api_key", name))
		}

		model := ""
		if v.IsSet(fmt.Sprintf("providers.%s.model", name)) {
			model = v.GetString(fmt.Sprintf("providers.%s.model", name))
		}

		configs = append(configs, core.ProviderConfig{
			Name:    name,
			APIKey:  apiKey,
			Model:   model,
			BaseURL: v.GetString(fmt.Sprintf("providers.%s.base_url", name)),
		})
	}

	return configs
}

func defaultSessionPath() string {
	home := os.Getenv("HOME")
	if home == "" {
		return filepath.Join(os.TempDir(), appName, "sessions")
	}
	return filepath.Join(home, ".local", "share", appName)
}

func Save(path string, cfg *core.Config) error {
	v := viper.New()
	v.SetConfigType("yaml")

	v.Set("provider", cfg.Provider)
	v.Set("ui", cfg.UI)
	v.Set("session", cfg.Session)
	v.Set("cache", cfg.Cache)
	v.Set("logging", cfg.Logging)

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	return v.WriteConfigAs(path)
}

func ResolveAPIKey(providerName string, cfg *core.Config) string {
	envKeys := map[string]string{
		"openai":     "OPENAI_API_KEY",
		"anthropic":  "ANTHROPIC_API_KEY",
		"gemini":     "GEMINI_API_KEY",
		"groq":       "GROQ_API_KEY",
		"nvidia":     "NVIDIA_API_KEY",
		"openrouter": "OPENROUTER_API_KEY",
	}

	// 1. Check environment variable for this specific provider.
	if k := os.Getenv(envKeys[providerName]); k != "" {
		return k
	}

	// 2. Check config for this specific provider.
	for _, pc := range cfg.Providers {
		if pc.Name == providerName && pc.APIKey != "" {
			return pc.APIKey
		}
	}

	return ""
}


