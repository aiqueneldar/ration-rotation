package config

import (
	"encoding/json"
	"log"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Database DatabaseConfig `json:"database"`
	Server   ServerConfig   `json:"server"`
	Signal   SignalConfig   `json:"signal"`
	Warning  WarningConfig  `json:"warning"`
}

type DatabaseConfig struct {
	URL                   string `json:"url"`
	CreateTablesIfMissing bool   `json:"create_tables_if_missing"`
}

type ServerConfig struct {
	Address     string       `json:"address"`
	Port        int          `json:"port"`
	CORSOrigins []string     `json:"cors_allowed_origins"`
	TZ          string       `json:"tz"`
	Domain      DomainConfig `json:"domain"`
}

type DomainConfig struct {
	Base   string `json:"base"`
	Prefix string `json:"prefix"`
	Full   string `json:"full,omitempty"`
}

type SignalConfig struct {
	Endpoint string `json:"endpoint"`
	To       string `json:"to"`
}

type WarningConfig struct {
	CheckIntervalMinutes int `json:"check_interval_minutes"`
}

var LoadedConfig Config

func Load() (*Config, error) {
	// Try loading from file first
	configPath := "config.json"
	if data, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(data, &LoadedConfig); err != nil {
			return nil, err
		}
	} else {
		log.Fatal(err)
	}

	// Apply environment variable overrides
	if url := os.Getenv("DATABASE_URL"); url != "" {
		LoadedConfig.Database.URL = url
	}

	if domain_base := os.Getenv("DOMAIN_BASE"); domain_base != "" {
		LoadedConfig.Server.Domain.Base = domain_base
	}

	if domain_prefix := os.Getenv("DOMAIN_PREFIX"); domain_prefix != "" {
		LoadedConfig.Server.Domain.Prefix = domain_prefix
	}

	if createTablesStr := os.Getenv("CREATE_TABLES_IF_MISSING"); createTablesStr != "" {
		createTables, _ := strconv.ParseBool(createTablesStr)
		LoadedConfig.Database.CreateTablesIfMissing = createTables
	}

	if addr := os.Getenv("SERVER_ADDRESS"); addr != "" {
		LoadedConfig.Server.Address = addr
	}

	if portStr := os.Getenv("SERVER_PORT"); portStr != "" {
		if port, _ := strconv.Atoi(portStr); port > 0 {
			LoadedConfig.Server.Port = port
		}
	}

	if originEnv := os.Getenv("CORS_ORIGINS"); originEnv != "" {
		for origin := range strings.SplitSeq(originEnv, ":") {
			LoadedConfig.Server.CORSOrigins = append(LoadedConfig.Server.CORSOrigins, origin)
		}
	}

	if endpoint := os.Getenv("SIGNAL_ENDPOINT"); endpoint != "" {
		LoadedConfig.Signal.Endpoint = endpoint
	}

	if intervalStr := os.Getenv("WARNING_CHECK_INTERVAL_MINUTES"); intervalStr != "" {
		if interval, _ := strconv.Atoi(intervalStr); interval > 0 {
			LoadedConfig.Warning.CheckIntervalMinutes = interval
		}
	}

	if tz := os.Getenv("TZ"); tz != "" {
		LoadedConfig.Server.TZ = tz
	} else {
		LoadedConfig.Server.TZ = "UTC"
	}

	if LoadedConfig.Warning.CheckIntervalMinutes >= 60 {
		LoadedConfig.Warning.CheckIntervalMinutes = 59
	}

	// Build full path, prefix might be nothing if we serve the server from the root of the web server
	if LoadedConfig.Server.Domain.Base != "" {
		LoadedConfig.Server.Domain.Full = LoadedConfig.Server.Domain.Base + LoadedConfig.Server.Domain.Prefix
	}

	return &LoadedConfig, nil
}
