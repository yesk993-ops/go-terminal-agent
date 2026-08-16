package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

	v.SetDefault("provider.default", "groq")
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
		for _, p := range configSearchPaths() {
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
			SavePath:    expandHomePath(v.GetString("session.save_path")),
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
		"gemini":     "GEMINI_API_KEY",
		"groq":       "GROQ_API_KEY",
		"nvidia":     "NVIDIA_API_KEY",
		"openrouter": "OPENROUTER_API_KEY",
	}

	shellKeys := loadShellEnv()

	var configs []core.ProviderConfig

	for name, envKey := range envMappings {
		model := ""
		if v.IsSet(fmt.Sprintf("providers.%s.model", name)) {
			model = v.GetString(fmt.Sprintf("providers.%s.model", name))
		}

		// Check for hardcoded key in config first.
		apiKey := ""
		if v.IsSet(fmt.Sprintf("providers.%s.api_key", name)) {
			raw := v.GetString(fmt.Sprintf("providers.%s.api_key", name))
			if !strings.HasPrefix(raw, "${") && raw != "" {
				apiKey = raw
			}
		}

		// Then shell/profile files (bashrc, PowerShell profile, agent.env).
		if apiKey == "" {
			if k, ok := shellKeys[envKey]; ok && k != "" {
				apiKey = k
			}
		}

		// Then environment variable (may be stale from parent shell).
		if apiKey == "" {
			apiKey = os.Getenv(envKey)
			if apiKey == "" && name == "gemini" {
				apiKey = os.Getenv("GOOGLE_API_KEY")
			}
		}

		// Last resort: unresolved ${ENV_VAR} references.
		if apiKey == "" && v.IsSet(fmt.Sprintf("providers.%s.api_key", name)) {
			apiKey = v.GetString(fmt.Sprintf("providers.%s.api_key", name))
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

// expandHomePath expands a leading "~" or "~/" (and "~\") to the user home
// directory. Paths without a tilde are returned unchanged. On Windows this
// correctly handles both forward and backslash separators.
func expandHomePath(path string) string {
	if path == "" {
		return path
	}
	if path == "~" {
		if home := userHome(); home != "" {
			return home
		}
		return path
	}
	// "~/" (Unix) or "~\" (Windows)
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		if home := userHome(); home != "" {
			return filepath.Join(home, path[2:])
		}
	}
	return path
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
		"gemini":     "GEMINI_API_KEY",
		"groq":       "GROQ_API_KEY",
		"nvidia":     "NVIDIA_API_KEY",
		"openrouter": "OPENROUTER_API_KEY",
	}

	// 1. Config file wins over everything (hardcoded keys).
	for _, pc := range cfg.Providers {
		if pc.Name == providerName && pc.APIKey != "" && !strings.HasPrefix(pc.APIKey, "${") {
			return pc.APIKey
		}
	}

	// 2. Shell / profile files — user's explicit config, overrides stale parent env.
	//    On Windows this also reads PowerShell profiles and agent.env.
	shellKeys := loadShellEnv()
	if k := shellKeys[envKeys[providerName]]; k != "" {
		return k
	}
	if providerName == "gemini" {
		if k := shellKeys["GOOGLE_API_KEY"]; k != "" {
			return k
		}
	}

	// 3. Environment variable (may be stale from parent shell).
	if k := os.Getenv(envKeys[providerName]); k != "" {
		return k
	}
	if providerName == "gemini" {
		if k := os.Getenv("GOOGLE_API_KEY"); k != "" {
			return k
		}
	}

	// 4. Config file with ${ENV_VAR} references.
	for _, pc := range cfg.Providers {
		if pc.Name == providerName && pc.APIKey != "" {
			return pc.APIKey
		}
	}

	return ""
}


