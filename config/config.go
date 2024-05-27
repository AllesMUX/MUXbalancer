package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

var (
	config *Config // in struct.go
)

func GetConfig(configFile string) *Config {
	if config != nil {
		return config
	}
	loadConfig(configFile)
	return config
}

func loadConfig(configFile string) {
	config = new(Config)
	yamlFile, err := os.ReadFile(configFile)
	if err != nil {
	    fmt.Println(err)
	}
	err = yaml.Unmarshal(yamlFile, config)
	if err != nil {
	    fmt.Println(err)
	}
}