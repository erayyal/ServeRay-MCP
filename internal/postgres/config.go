package postgres

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/erayyal/serveray-mcp/internal/shared/buildinfo"
	sharedconfig "github.com/erayyal/serveray-mcp/internal/shared/config"
	shareddb "github.com/erayyal/serveray-mcp/internal/shared/db"
)

const (
	Name    = "postgres-mcp"
	Version = buildinfo.Version
)

type Config struct {
	DB       shareddb.BaseConfig
	DSN      string
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string
}

func LoadConfig() (Config, error) {
	base, err := shareddb.LoadBaseConfig("POSTGRES")
	if err != nil {
		return Config{}, err
	}
	if err := base.ValidateWriteOptIn("POSTGRES"); err != nil {
		return Config{}, err
	}

	cfg := Config{
		DB:       base,
		DSN:      sharedconfig.String("POSTGRES_DSN", ""),
		Host:     sharedconfig.String("POSTGRES_HOST", "localhost"),
		Database: sharedconfig.String("POSTGRES_DATABASE", "postgres"),
		SSLMode:  sharedconfig.String("POSTGRES_SSLMODE", "require"),
	}

	cfg.Port, err = sharedconfig.Int("POSTGRES_PORT", 5432, 1, 65535)
	if err != nil {
		return Config{}, err
	}

	if cfg.DSN == "" {
		cfg.User, err = sharedconfig.RequiredString("POSTGRES_USER")
		if err != nil {
			return Config{}, err
		}
		cfg.Password, err = sharedconfig.RequiredString("POSTGRES_PASSWORD")
		if err != nil {
			return Config{}, err
		}
		cfg.DSN, err = cfg.buildDSN()
		if err != nil {
			return Config{}, err
		}
	}

	return cfg, nil
}

func (c Config) buildDSN() (string, error) {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.User, c.Password),
		Host:   fmt.Sprintf("%s:%d", c.Host, c.Port),
		Path:   c.Database,
	}

	query := u.Query()
	query.Set("sslmode", c.SSLMode)
	query.Set("connect_timeout", strconv.Itoa(int(c.DB.ConnectTimeout.Seconds())))
	query.Set("application_name", Name)
	if !c.DB.WriteEnabled() {
		query.Set("default_transaction_read_only", "on")
	}
	u.RawQuery = query.Encode()
	return u.String(), nil
}
