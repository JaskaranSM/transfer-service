package config

import (
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	Env string `mapstructure:"ENVIRONMENT"`

	// Fiber config
	Port    int  `mapstructure:"APP_PORT"`
	Prefork bool `mapstructure:"APP_PREFORK"`

	// Logging config
	LogLevel string `mapstructure:"LOG_LEVEL"`

	// Gdrive config
	UseSA bool `mapstructure:"USE_SA"`
}

var cfg *Config

func init() {
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	viper.AddConfigPath("../.")

	viper.SetDefault("APP_PORT", 6969)
	viper.SetDefault("LOG_LEVEL", "")
	viper.SetDefault("USE_SA", true)
	viper.SetDefault("ENVIRONMENT", "")
	viper.AutomaticEnv()

	// Read config file
	err := viper.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
		} else {
			log.Fatalln(err)
		}
	}

	// Set config object
	err = viper.Unmarshal(&cfg)
	if err != nil {
		log.Fatal(err)
	}

	// Validate that all config vars are set
	cfg.Validate()

	// Set default log level to debug if environment is local
	if cfg.Env == "local" && cfg.LogLevel == "" {
		cfg.LogLevel = "debug"
	}
}

func (cfg Config) Validate() {

}

func Get() *Config {
	if cfg == nil {
		log.Fatalln("Config not set ^._.^")
	}
	return cfg
}
