package config

type Balance struct {
    Path   string `yaml:"path"`
    Method  string `yaml:"method"`
}

type WorkerConfig struct {
    HealthEndpoint string `yaml:"health"`
    Balance []Balance `yaml:"balance"`
}

type AppConfig struct {
    Serve      string `yaml:"serve"`
    Port       int    `yaml:"port"`
    Socket     string `yaml:"socket"`
    Cookie     string `yaml:"cookie"`
    SessionLifetime int `yaml:"session_lifetime"`
}

type RedisConfig struct {
    Addr  string `yaml:"addr"`
    Password string `yaml:"password"`
    DB    int    `yaml:"db"`
}

type Config struct {
    App    AppConfig  `yaml:"app"`
    Redis  RedisConfig `yaml:"redis"`
    Worker WorkerConfig  `yaml:"worker"`
}
