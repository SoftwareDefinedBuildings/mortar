package main

import (
	"github.com/spf13/viper"
)

type Config struct {
	// configuration for amazon cognito
	Cognito CognitoAuthConfig

	ListenAddr string
	BTrDBAddr  string
}
type CognitoAuthConfig struct {
	// the client identifier for the app
	AppClientId string
	// the client secret
	AppClientSecret string
	// id of the user pool
	PoolId string
	// the .well-known/jwks.json URL
	JWKUrl string
	// region to query for the user pool
	Region string
}

func getCfg() *Config {
	cognito := CognitoAuthConfig{
		AppClientId:     viper.GetString("Cognito.AppClientId"),
		AppClientSecret: viper.GetString("Cognito.AppClientSecret"),
		PoolId:          viper.GetString("Cognito.PoolId"),
		JWKUrl:          viper.GetString("Cognito.JWKUrl"),
		Region:          viper.GetString("Cognito.Region"),
	}

	return &Config{
		Cognito:    cognito,
		ListenAddr: viper.GetString("ListenAddr"),
		BTrDBAddr:  viper.GetString("BTrDBAddr"),
	}
}

func ReadConfig(file string) (*Config, error) {
	if len(file) > 0 {
		viper.SetConfigFile(file)
	}
	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	return getCfg(), nil
}
