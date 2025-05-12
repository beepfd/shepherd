package client

import (
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/cen-ngc5139/shepherd/internal/config"
)

func NewClickHouseConn(cfg config.ClickhouseOutputConfig) (clickhouse.Conn, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Protocol: clickhouse.HTTP,
		Addr:     []string{fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)},
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		MaxIdleConns: 5,
		MaxOpenConns: 10,
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
		ConnMaxLifetime:  time.Hour * 3,
		ConnOpenStrategy: clickhouse.ConnOpenInOrder,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to clickhouse: %v", err)
	}
	return conn, nil
}
