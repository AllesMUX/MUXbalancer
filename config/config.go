package config

import (
	"fmt"
	"github.com/spf13/viper"
)


func InitConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()

	viper.BindEnv("app.port", "MUX_APP_PORT")
	viper.BindEnv("api.port", "MUX_API_PORT")
	viper.BindEnv("api.token", "MUX_API_TOKEN")
	viper.BindEnv("redis.host", "REDIS_HOST")
	viper.BindEnv("redis.port", "REDIS_ADDR")
	viper.BindEnv("redis.password", "REDIS_PASSWORD")
	viper.BindEnv("redis.db", "REDIS_DB")
	viper.BindEnv("app.serve", "MUX_APP_SERVE")
	viper.BindEnv("app.socket", "MUX_APP_SOCKET")
	viper.BindEnv("app.cookie", "MUX_APP_COOKIE")
	viper.BindEnv("app.session_lifetime", "MUX_APP_SESSION_LIFETIME")
	viper.BindEnv("worker.health", "MUX_WORKER_HEALTH")

	var config Config

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file, %s", err)
		}
	}
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unable to decode into struct, %v", err)
	}
	return &config, nil
}