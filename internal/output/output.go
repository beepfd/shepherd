package output

import (
	"context"
	"encoding/json"

	"github.com/ClickHouse/clickhouse-go/v2"
	ckdriver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/cen-ngc5139/shepherd/internal/binary"
	"github.com/cen-ngc5139/shepherd/internal/config"
	"github.com/cen-ngc5139/shepherd/internal/log"
	"github.com/cen-ngc5139/shepherd/pkg/client"
	"github.com/cen-ngc5139/shepherd/pkg/kafka"
	"github.com/pkg/errors"
)

type SinkCli struct {
	CKCli    CKCli
	KafkaCli *kafka.Producer
}

type CKCli struct {
	conn    clickhouse.Conn
	batch   ckdriver.Batch
	counter int
}

type Output struct {
	SinkType config.OutputType
	SinkCli  SinkCli
	ctx      context.Context
}

func NewOutput(cfg config.Configuration, ctx context.Context) (*Output, error) {
	o := &Output{SinkType: cfg.Output.Type, ctx: ctx}
	if err := o.InitSinkCli(cfg.Output); err != nil {
		return nil, errors.Wrapf(err, "failed to init sink %s client", o.SinkType)
	}

	return o, nil
}

func (o *Output) Close() {
	if o.SinkType == config.OutputTypeClickhouse {
		log.Info("close clickhouse client")
		o.SinkCli.CKCli.conn.Close()
	}
}

func (o *Output) InitSinkCli(cfg config.OutputConfig) (err error) {
	if o.SinkType == config.OutputTypeClickhouse {
		conn, err := client.NewClickHouseConn(cfg.Clickhouse)
		if err != nil {
			return errors.Wrap(err, "failed to init clickhouse client")
		}

		o.SinkCli.CKCli.batch, err = conn.PrepareBatch(o.ctx, `
			INSERT INTO sched_latency (
				pid, tid, delay_ns, ts, 
				preempted_pid, preempted_comm, 
				is_preempt, comm
			)
		`)
		if err != nil {
			return errors.Wrap(err, "failed to prepare batch")
		}

		o.SinkCli.CKCli.conn = conn
	}

	if o.SinkType == config.OutputTypeKafka {
		o.SinkCli.KafkaCli, err = kafka.NewSyncProducer(config.Config.Output.Kafka.Brokers, config.Config.Output.Kafka.Topic, true, true)
		if err != nil {
			return errors.Wrap(err, "failed to init kafka client")
		}
	}

	return nil
}

func (o *Output) Push(event binary.ShepherdSchedLatencyT) error {
	if o.SinkType == config.OutputTypeClickhouse {
		batch, count, err := insertSchedMetrics(o.ctx, o.SinkCli.CKCli.conn, o.SinkCli.CKCli.batch, event, o.SinkCli.CKCli.counter)
		if err != nil {
			return errors.Wrap(err, "failed to insert sched metrics")
		}
		o.SinkCli.CKCli.batch = batch
		o.SinkCli.CKCli.counter = count
	}

	if o.SinkType == config.OutputTypeKafka {
		raw, err := json.Marshal(event)
		if err != nil {
			return errors.Wrap(err, "failed to marshal event")
		}

		_, _, err = o.SinkCli.KafkaCli.SyncSendMessage(raw)
		if err != nil {
			return errors.Wrap(err, "fail to push kafka data")
		}
	}

	log.StdoutOrFile("file", event)

	return nil
}
