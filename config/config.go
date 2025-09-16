package config

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	GiftsOffset        uint32   `mapstructure:"gifts_offset"`
	GiftsPerFetch      uint32   `mapstructure:"gifts_per_fetch"`
	ConcurrentRequests int      `mapstructure:"concurrent_requests"`
	MinProfit          float64  `mapstructure:"min_profit"`
	MinProfitTon       float64  `mapstructure:"min_profit_ton"`
	RareBackdrops      []string `mapstructure:"rare_backdrops"`

	Proxies []string `mapstructure:"proxies"`
	Token   string   `mapstructure:"token"`
	ChatID  int64    `mapstructure:"chat_id"`
}

func LoadConfig(path string) (*Config, error) {
	viper.AddConfigPath(path)
	viper.SetConfigName("config")
	viper.SetConfigType("json")

	// Set defaults (mirror constants)
	viper.SetDefault("gifts_offset", 0)
	viper.SetDefault("gifts_per_fetch", 30)
	viper.SetDefault("concurrent_requests", 5)
	viper.SetDefault("min_profit", 0.02)
	viper.SetDefault("min_profit_ton", 0.0)
	viper.SetDefault("rare_backdrops", []string{"Black"})
	viper.SetDefault("proxies", []string{})

	// Enable reading from environment variables
	viper.AutomaticEnv()
	viper.BindEnv("token", "TOKEN")
	viper.BindEnv("chat_id", "CHAT_ID")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if env := os.Getenv("PROXIES"); env != "" {
		parts := strings.Split(env, ",")
		cfg.Proxies = nil
		for _, p := range parts {
			proxyStr := strings.TrimSpace(p)
			if proxyStr == "" {
				continue
			}
			if _, err := url.Parse(proxyStr); err != nil {
				return nil, fmt.Errorf("invalid proxy URL in PROXIES env: %s", proxyStr)
			}
			cfg.Proxies = append(cfg.Proxies, proxyStr)
		}
	}

	log.Printf("Loaded %d proxies\n", len(cfg.Proxies))

	if cfg.Token == "" {
		return nil, fmt.Errorf("TOKEN env is required")
	}
	if cfg.ChatID == 0 {
		return nil, fmt.Errorf("CHAT_ID env is required and must be a valid integer")
	}

	return &cfg, nil
}
