package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	BrokerURL string `mapstructure:"broker_url"`
}

var Conf Config

func init() {
	viper.SetConfigName("config")   // name of config file (without extension)
	viper.SetConfigType("yaml")     // REQUIRED if the config file does not have the extension in the name
	viper.AddConfigPath("./config") // optionally look for config in the working directory
	err := viper.ReadInConfig()     // Find and read the config file
	if err != nil {                 // Handle errors reading the config file
		panic(fmt.Errorf("fatal error config file: %w", err))
	}
	if err := viper.Unmarshal(&Conf); err != nil {
		panic(fmt.Errorf("fatal error config file: %w", err))
	}
}
