package client

import (
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/cen-ngc5139/shepherd/internal/config"
)

func NewClickHouseConn(cfg config.Configuration, db string) (clickhouse.Conn, error) {
	ckCfg := cfg.Output.Clickhouse

	conn, err := clickhouse.Open(&clickhouse.Options{
		Protocol: clickhouse.HTTP,
		Addr:     []string{fmt.Sprintf("%s:%s", ckCfg.Host, ckCfg.Port)},
		Auth: clickhouse.Auth{
			Database: db,
			Username: ckCfg.Username,
			Password: ckCfg.Password,
		},
		MaxIdleConns: 5,
		MaxOpenConns: 10,
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
		ConnMaxLifetime:  3600,
		ConnOpenStrategy: clickhouse.ConnOpenInOrder,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to clickhouse: %v", err)
	}
	return conn, nil
}
