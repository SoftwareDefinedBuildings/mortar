package stages

import (
	"github.com/spf13/viper"
	"os"
)

type Config struct {
	// configuration for amazon cognito
	Cognito CognitoAuthConfig

	HodConfig      string
	ListenAddr     string
	BTrDBAddr      string
	PrometheusAddr string

	TLSCrtFile string
	TLSKeyFile string
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
	viper.SetDefault("Cognito.AppClientId", os.Getenv("COGNITO_APP_CLIENT_ID"))
	viper.SetDefault("Cognito.AppClientSecret", os.Getenv("COGNITO_APP_CLIENT_SECRET"))
	viper.SetDefault("Cognito.PoolId", os.Getenv("COGNITO_POOL_ID"))
	viper.SetDefault("Cognito.JWKUrl", os.Getenv("COGNITO_JWK_URL"))
	viper.SetDefault("Cognito.Region", os.Getenv("COGNITO_REGION"))
	viper.SetDefault("Cognito.Region", os.Getenv("COGNITO_REGION"))
	viper.SetDefault("HodConfig", os.Getenv("HODCONFIG_LOCATION"))
	viper.SetDefault("BTrDBAddr", os.Getenv("BTRDB_ADDRESS"))
	viper.SetDefault("ListenAddr", os.Getenv("LISTEN_ADDRESS"))
	viper.SetDefault("PrometheusAddr", os.Getenv("PROMETHEUS_ADDRESS"))
	viper.SetDefault("TLSCrtFile", os.Getenv("MORTAR_TLS_CRT_FILE"))
	viper.SetDefault("TLSKeyFile", os.Getenv("MORTAR_TLS_KEY_FILE"))

	log.Warning(viper.GetString("BTrDBAddr"))
	cognito := CognitoAuthConfig{
		AppClientId:     viper.GetString("Cognito.AppClientId"),
		AppClientSecret: viper.GetString("Cognito.AppClientSecret"),
		PoolId:          viper.GetString("Cognito.PoolId"),
		JWKUrl:          viper.GetString("Cognito.JWKUrl"),
		Region:          viper.GetString("Cognito.Region"),
	}

	return &Config{
		Cognito:        cognito,
		HodConfig:      viper.GetString("HodConfig"),
		ListenAddr:     viper.GetString("ListenAddr"),
		BTrDBAddr:      viper.GetString("BTrDBAddr"),
		PrometheusAddr: viper.GetString("PrometheusAddr"),
		TLSCrtFile:     viper.GetString("TLSCrtFile"),
		TLSKeyFile:     viper.GetString("TLSKeyFile"),
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
