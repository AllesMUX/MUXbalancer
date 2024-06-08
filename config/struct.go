package config

type Balance struct {
	Path   string `mapstructure:"path"`
	Method string `mapstructure:"method"`
}

type WorkerConfig struct {
	HealthEndpoint string    `mapstructure:"health"`
	Balance        []Balance `mapstructure:"balance"`
}

type AppConfig struct {
	Serve           string `mapstructure:"serve"`
	Port            int    `mapstructure:"port"`
	Socket          string `mapstructure:"socket"`
	Cookie          string `mapstructure:"cookie"`
	SessionLifetime int    `mapstructure:"session_lifetime"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type ApiConfig struct {
	Token string `mapstructure:"token"`
	Port  int    `mapstructure:"port"`
}

type Config struct {
	App    AppConfig    `mapstructure:"app"`
	Redis  RedisConfig  `mapstructure:"redis"`
	Worker WorkerConfig `mapstructure:"worker"`
	API    ApiConfig    `mapstructure:"api"`
}
