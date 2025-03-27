package config

import (
	"log"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server ServerConfig `mapstructure:"SERVER"`
	DB     DBConfig     `mapstructure:"DB"`
	JWT    JWTConfig    `mapstructure:"JWT"`
	Redis  RedisConfig  `mapstructure:"REDIS"`
}

type ServerConfig struct {
	Port         string        `mapstructure:"PORT"`
	ReadTimeout  time.Duration `mapstructure:"READ_TIMEOUT"`
	WriteTimeout time.Duration `mapstructure:"WRITE_TIMEOUT"`
}

type DBConfig struct {
	DSN          string `mapstructure:"DSN"`
	MaxOpenConns int    `mapstructure:"MAX_OPEN_CONNS"`
}

type JWTConfig struct {
	Secret       string        `mapstructure:"SECRET"`
	AccessExpiry time.Duration `mapstructure:"ACCESS_EXPIRY"`
	RefreshExpiry time.Duration `mapstructure:"REFRESH_EXPIRY"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"ADDR"`
	Password string `mapstructure:"PASSWORD"`
	DB       int    `mapstructure:"DB"`
}

func LoadConfig() *Config {
	viper.SetConfigFile(".env")
	viper.AutomaticEnv()

	// Set defaults
	viper.SetDefault("SERVER.PORT", "3000")
	viper.SetDefault("SERVER.READ_TIMEOUT", "10s")
	viper.SetDefault("SERVER.WRITE_TIMEOUT", "10s")
	viper.SetDefault("DB.DSN", "postgres://auth_user:auth_pass@postgres:5432/auth_db?sslmode=disable")
	viper.SetDefault("DB.MAX_OPEN_CONNS", 25)
	viper.SetDefault("JWT.SECRET", "default-secret-change-me")
	viper.SetDefault("JWT.ACCESS_EXPIRY", "15m")
	viper.SetDefault("JWT.REFRESH_EXPIRY", "168h") // 7 days
	viper.SetDefault("REDIS.ADDR", "redis:6379")
	viper.SetDefault("REDIS.PASSWORD", "")
	viper.SetDefault("REDIS.DB", 0)

	// Try to read .env file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Println("No .env file found, using defaults and environment variables")
		} else {
			log.Printf("Error reading config file: %v", err)
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		log.Fatalf("Unable to decode into struct: %v", err)
	}

	// Convert string durations to time.Duration if needed
	if _, err := time.ParseDuration(cfg.Server.ReadTimeout.String()); err != nil {
		cfg.Server.ReadTimeout = 10 * time.Second
	}
	if _, err := time.ParseDuration(cfg.Server.WriteTimeout.String()); err != nil {
		cfg.Server.WriteTimeout = 10 * time.Second
	}
	if _, err := time.ParseDuration(cfg.JWT.AccessExpiry.String()); err != nil {
		cfg.JWT.AccessExpiry = 15 * time.Minute
	}
	if _, err := time.ParseDuration(cfg.JWT.RefreshExpiry.String()); err != nil {
		cfg.JWT.RefreshExpiry = 168 * time.Hour
	}

	return &cfg
}
