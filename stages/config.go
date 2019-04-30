package stages

import (
	"github.com/spf13/viper"
	"os"
)

type Config struct {
	// configuration for amazon cognito
	Cognito CognitoAuthConfig

	// wavemq frontend config
	WAVEMQ WAVEMQConfig

	// WAVE auth frontend config
	WAVE WAVEConfig

	HodConfig      string
	ListenAddr     string
	BTrDBAddr      string
	InfluxDBAddr   string
	InfluxDBUser   string
	InfluxDBPass   string
	PrometheusAddr string

	TLSCrtFile string
	TLSKeyFile string
}

type WAVEConfig struct {
	// defaults to localhost:410
	Agent string
	// defaults to WAVE_DEFAULT_ENTITY
	EntityFile string
	// proof file for esrver
	ProofFile string
}

type WAVEMQConfig struct {
	// defaults to localhost:4516
	SiteRouter string
	// defaults to WAVE_DEFAULT_ENTITY
	EntityFile string
	// namespace this is hosted on
	Namespace string
	// resource prefix for the server
	BaseURI string
	// name of the mortar service
	ServerName string
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

	viper.SetDefault("WAVEMQ.SiteRouter", "localhost:4516")
	viper.SetDefault("WAVEMQ.EntityFile", os.Getenv("WAVE_DEFAULT_ENTITY"))
	viper.SetDefault("WAVEMQ.Namespace", os.Getenv("MORTAR_WAVE_NAMESPACE"))
	viper.SetDefault("WAVEMQ.BaseURI", os.Getenv("MORTAR_WAVE_BASEURI"))
	viper.SetDefault("WAVEMQ.ServerName", os.Getenv("MORTAR_WAVE_SERVERNAME"))

	viper.SetDefault("WAVE.Agent", "localhost:410")
	viper.SetDefault("WAVE.EntityFile", os.Getenv("WAVE_DEFAULT_ENTITY"))
	viper.SetDefault("WAVE.ProofFile", os.Getenv("MORTAR_WAVE_SERVERPROOF"))

	viper.SetDefault("HodConfig", os.Getenv("HODCONFIG_LOCATION"))
	viper.SetDefault("BTrDBAddr", os.Getenv("BTRDB_ADDRESS"))
	viper.SetDefault("InfluxDBAddr", os.Getenv("INFLUXDB_ADDRESS"))
	viper.SetDefault("InfluxDBUser", os.Getenv("INFLUXDB_USER"))
	viper.SetDefault("InfluxDBPass", os.Getenv("INFLUXDB_PASS"))
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

	wavemqcfg := WAVEMQConfig{
		SiteRouter: viper.GetString("WAVEMQ.SiteRouter"),
		EntityFile: viper.GetString("WAVEMQ.EntityFile"),
		Namespace:  viper.GetString("WAVEMQ.Namespace"),
		BaseURI:    viper.GetString("WAVEMQ.BaseURI"),
		ServerName: viper.GetString("WAVEMQ.ServerName"),
	}

	wavecfg := WAVEConfig{
		EntityFile: viper.GetString("WAVE.EntityFile"),
		ProofFile:  viper.GetString("WAVE.ProofFile"),
		Agent:      viper.GetString("WAVE.Agent"),
	}

	return &Config{
		Cognito:        cognito,
		WAVEMQ:         wavemqcfg,
		WAVE:           wavecfg,
		HodConfig:      viper.GetString("HodConfig"),
		ListenAddr:     viper.GetString("ListenAddr"),
		BTrDBAddr:      viper.GetString("BTrDBAddr"),
		InfluxDBAddr:   viper.GetString("InfluxDBAddr"),
		InfluxDBUser:   viper.GetString("InfluxDBUser"),
		InfluxDBPass:   viper.GetString("InfluxDBPass"),
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
