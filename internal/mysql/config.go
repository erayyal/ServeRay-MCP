package mysql

import (
	"fmt"

	driver "github.com/go-sql-driver/mysql"

	"github.com/erayyal/serveray-mcp/internal/shared/buildinfo"
	sharedconfig "github.com/erayyal/serveray-mcp/internal/shared/config"
	shareddb "github.com/erayyal/serveray-mcp/internal/shared/db"
)

const (
	Name    = "mysql-mcp"
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
	TLS      string
}

func LoadConfig() (Config, error) {
	base, err := shareddb.LoadBaseConfig("MYSQL")
	if err != nil {
		return Config{}, err
	}
	if err := base.ValidateWriteOptIn("MYSQL"); err != nil {
		return Config{}, err
	}

	cfg := Config{
		DB:       base,
		DSN:      sharedconfig.String("MYSQL_DSN", ""),
		Host:     sharedconfig.String("MYSQL_HOST", "localhost"),
		Database: sharedconfig.String("MYSQL_DATABASE", ""),
		TLS:      sharedconfig.String("MYSQL_TLS", "true"),
	}

	cfg.Port, err = sharedconfig.Int("MYSQL_PORT", 3306, 1, 65535)
	if err != nil {
		return Config{}, err
	}

	if cfg.DSN == "" {
		cfg.User, err = sharedconfig.RequiredString("MYSQL_USER")
		if err != nil {
			return Config{}, err
		}
		cfg.Password, err = sharedconfig.RequiredString("MYSQL_PASSWORD")
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
	driverConfig := driver.NewConfig()
	driverConfig.User = c.User
	driverConfig.Passwd = c.Password
	driverConfig.Net = "tcp"
	driverConfig.Addr = fmt.Sprintf("%s:%d", c.Host, c.Port)
	driverConfig.DBName = c.Database
	driverConfig.ParseTime = true
	driverConfig.Params = map[string]string{
		"maxAllowedPacket": "0",
	}
	driverConfig.Timeout = c.DB.ConnectTimeout
	driverConfig.ReadTimeout = c.DB.QueryTimeout
	driverConfig.WriteTimeout = c.DB.QueryTimeout
	driverConfig.TLSConfig = c.TLS
	return driverConfig.FormatDSN(), nil
}
