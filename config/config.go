package config

import (
	"time"

	"github.com/spf13/viper"
)

var cfg *config

type config struct {
	IpRequestLimit    int           `mapstructure:"IP_REQUEST_LIMIT"`
	TokenRequestLimit int           `mapstructure:"TOKEN_REQUEST_LIMIT"`
	TimeWindow        time.Duration `mapstructure:"TIME_WINDOW"`
	BlockTime         time.Duration `mapstructure:"BLOCK_TIME"`
	SecretKey         string        `mapstructure:"SECRET_KEY"`
	RedisAddr         string        `mapstructure:"REDIS_ADDR"`
	RedisPassword     string        `mapstructure:"REDIS_PASSWORD"`
}

func LoadConfig(path string) *config {
	viper.SetConfigName("app_config")
	viper.SetConfigType("env")
	viper.AddConfigPath(path)
	viper.SetConfigFile(".env")
	viper.AutomaticEnv()
	err := viper.ReadInConfig()
	if err != nil {
		panic(err)
	}
	err = viper.Unmarshal(&cfg)
	if err != nil {
		panic(err)
	}
	return cfg
}
