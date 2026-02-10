package config

import (
	"log"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	RedisURL      string  `mapstructure:"redis_url"`
	Queue         string  `mapstructure:"queue"`
	Interval      float64 `mapstructure:"interval_s"`
	GroupsN       int     `mapstructure:"groups_n"`
	GroupPrefix   string  `mapstructure:"group_prefix"`
	AutoDiscover  bool    `mapstructure:"auto_discover"`
	DiscoverLimit int     `mapstructure:"discover_limit"`
}

func LoadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("toml")
	viper.AddConfigPath(".")

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	viper.SetDefault("redis_url", "redis://localhost:6379")
	viper.SetDefault("interval_s", 0.2)
	viper.SetDefault("groups_n", 4)
	viper.SetDefault("group_prefix", "g")
	viper.SetDefault("auto_discover", true)
	viper.SetDefault("discover_limit", 200)

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
		log.Println("No config.toml found, using defaults and environment variables")
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
