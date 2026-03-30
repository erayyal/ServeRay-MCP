package mssql

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/erayyal/serveray-mcp/internal/shared/buildinfo"
	sharedconfig "github.com/erayyal/serveray-mcp/internal/shared/config"
	shareddb "github.com/erayyal/serveray-mcp/internal/shared/db"
)

const (
	Name    = "mssql-mcp"
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
	Encrypt  string
}

func LoadConfig() (Config, error) {
	base, err := shareddb.LoadBaseConfig("MSSQL")
	if err != nil {
		return Config{}, err
	}
	if err := base.ValidateWriteOptIn("MSSQL"); err != nil {
		return Config{}, err
	}

	cfg := Config{
		DB:       base,
		DSN:      sharedconfig.String("MSSQL_DSN", ""),
		Host:     sharedconfig.String("MSSQL_HOST", "localhost"),
		Database: sharedconfig.String("MSSQL_DATABASE", ""),
		Encrypt:  sharedconfig.String("MSSQL_ENCRYPT", "true"),
	}

	cfg.Port, err = sharedconfig.Int("MSSQL_PORT", 1433, 1, 65535)
	if err != nil {
		return Config{}, err
	}

	if cfg.DSN == "" {
		cfg.User, err = sharedconfig.RequiredString("MSSQL_USER")
		if err != nil {
			return Config{}, err
		}
		cfg.Password, err = sharedconfig.RequiredString("MSSQL_PASSWORD")
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
		Scheme: "sqlserver",
		User:   url.UserPassword(c.User, c.Password),
		Host:   fmt.Sprintf("%s:%d", c.Host, c.Port),
	}

	query := u.Query()
	if c.Database != "" {
		query.Set("database", c.Database)
	}
	query.Set("encrypt", c.Encrypt)
	query.Set("connection timeout", strconv.Itoa(int(c.DB.ConnectTimeout.Seconds())))
	query.Set("app name", Name)
	u.RawQuery = query.Encode()

	return u.String(), nil
}
