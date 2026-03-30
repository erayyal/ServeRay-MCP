package db

import (
	"fmt"
	"strings"
	"time"

	"github.com/erayyal/serveray-mcp/internal/shared/config"
)

func LoadBaseConfig(prefix string) (BaseConfig, error) {
	connectTimeout, err := config.Duration(prefix+"_CONNECT_TIMEOUT", 10*time.Second)
	if err != nil {
		return BaseConfig{}, err
	}
	queryTimeout, err := config.Duration(prefix+"_QUERY_TIMEOUT", 15*time.Second)
	if err != nil {
		return BaseConfig{}, err
	}
	maxOpenConns, err := config.Int(prefix+"_MAX_OPEN_CONNS", 4, 1, 20)
	if err != nil {
		return BaseConfig{}, err
	}
	maxIdleConns, err := config.Int(prefix+"_MAX_IDLE_CONNS", 2, 1, 20)
	if err != nil {
		return BaseConfig{}, err
	}
	connMaxLifetime, err := config.Duration(prefix+"_CONN_MAX_LIFETIME", 10*time.Minute)
	if err != nil {
		return BaseConfig{}, err
	}
	connMaxIdleTime, err := config.Duration(prefix+"_CONN_MAX_IDLE_TIME", 5*time.Minute)
	if err != nil {
		return BaseConfig{}, err
	}
	maxRows, err := config.Int(prefix+"_MAX_ROWS", 200, 1, 1000)
	if err != nil {
		return BaseConfig{}, err
	}
	maxBytes, err := config.Int(prefix+"_MAX_BYTES", 131072, 1024, 1048576)
	if err != nil {
		return BaseConfig{}, err
	}
	maxCellChars, err := config.Int(prefix+"_MAX_CELL_CHARS", 2048, 64, 8192)
	if err != nil {
		return BaseConfig{}, err
	}
	enableWrite, err := config.Bool(prefix+"_ENABLE_WRITE", false)
	if err != nil {
		return BaseConfig{}, err
	}

	return BaseConfig{
		ConnectTimeout:  connectTimeout,
		QueryTimeout:    queryTimeout,
		MaxOpenConns:    maxOpenConns,
		MaxIdleConns:    maxIdleConns,
		ConnMaxLifetime: connMaxLifetime,
		ConnMaxIdleTime: connMaxIdleTime,
		MaxRows:         maxRows,
		MaxBytes:        maxBytes,
		MaxCellChars:    maxCellChars,
		EnableWrite:     enableWrite,
		WriteAck:        strings.TrimSpace(config.String(prefix+"_WRITE_ACK", "")),
	}, nil
}

func (c BaseConfig) ValidateWriteOptIn(prefix string) error {
	if c.EnableWrite && !c.WriteEnabled() {
		return fmt.Errorf("%s_ENABLE_WRITE requires %s_WRITE_ACK=ENABLE_UNSAFE_WRITE_OPERATIONS", prefix, prefix)
	}
	return nil
}
