package output

import (
	"context"
	"os"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/cen-ngc5139/shepherd/internal/binary"
	"github.com/cen-ngc5139/shepherd/internal/cache"
	"github.com/cen-ngc5139/shepherd/internal/config"
	"github.com/cen-ngc5139/shepherd/internal/log"
	"github.com/cen-ngc5139/shepherd/internal/metadata"
	"github.com/cen-ngc5139/shepherd/pkg/client"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/perf"
)

func ProcessSchedDelay(coll *ebpf.Collection, ctx context.Context, cfg config.Configuration) {
	schedEvents := coll.Maps["sched_events"]
	perfReader, err := perf.NewReader(schedEvents, os.Getpagesize())
	if err != nil {
		log.Errorf("failed to create ringbuf reader: %v", err)
		return
	}

	defer perfReader.Close()

	conn, err := client.NewClickHouseConn(cfg, cfg.Output.Clickhouse.Database)
	if err != nil {
		log.Fatalf("failed to connect to clickhouse: %v", err)
	}

	// defer conn.Close()

	// 准备批量插入语句
	batch, err := conn.PrepareBatch(ctx, `
        INSERT INTO sched_latency (
            pid, tid, delay_ns, ts, 
            preempted_pid, preempted_comm, 
            is_preempt, comm
        )
    `)
	if err != nil {
		log.Fatalf("failed to prepare batch: %v", err)
	}

	// 添加静态计数器
	var count int

	var event binary.ShepherdSchedLatencyT
	for {
		// 在循环开始时就检查 context
		select {
		case <-ctx.Done():
			log.Info("退出事件处理")
			return
		default:
			if err := parseEvent(perfReader, &event); err != nil {
				log.Errorf("failed to parse perf event: %v", err)
				continue
			}

			batch, count, err = insertSchedMetrics(ctx, conn, batch, event, count)
			if err != nil {
				log.Errorf("failed to insert sched metrics: %v", err)
				continue
			}

			schedMetrics := metadata.SchedMetrics{
				Pid:     event.Pid,
				DelayNs: event.DelayNs,
				Ts:      event.Ts,
				Comm:    sanitizeString(convertInt8ToString(event.Comm[:])),
			}

			current, isExist := cache.SchedMetricsMap.Load(event.Pid)
			if !isExist {
				cache.SchedMetricsMap.Store(event.Pid, schedMetrics)
				continue
			}

			currentSchedMetrics, ok := current.(metadata.SchedMetrics)
			if !ok {
				log.Errorf("failed to convert current to metadata.SchedMetrics: %v", current)
				continue
			}

			currentSchedMetrics.DelayNs = event.DelayNs + currentSchedMetrics.DelayNs
			if event.IsPreempt != 1 {
				cache.SchedMetricsMap.Store(event.Pid, currentSchedMetrics)
				continue
			}

			currentSchedMetrics.PreempteCount++
			schedPreempted := metadata.SchedPreempted{

				Pid:   event.PreemptedPid,
				Count: 1,
				Comm:  sanitizeString(convertInt8ToString(event.PreemptedComm[:])),
			}

			preempted, isExist := cache.SchedPreemptedMap.Load(event.PreemptedPid)
			if !isExist {
				cache.SchedPreemptedMap.Store(event.PreemptedPid, schedPreempted)
				continue
			}

			preemptedSchedMetrics, ok := preempted.(metadata.SchedPreempted)
			if !ok {
				log.Errorf("failed to convert preempted to metadata.SchedPreempted: %v", preempted)
				continue
			}

			preemptedSchedMetrics.Count++
			cache.SchedPreemptedMap.Store(event.PreemptedPid, preemptedSchedMetrics)
			cache.SchedMetricsMap.Store(event.Pid, currentSchedMetrics)
		}
	}

}

func insertSchedMetrics(ctx context.Context, conn clickhouse.Conn, batch driver.Batch, event binary.ShepherdSchedLatencyT, count int) (driver.Batch, int, error) {
	err := batch.Append(
		event.Pid,
		event.Tid,
		event.DelayNs,
		event.Ts,
		event.PreemptedPid,
		sanitizeString(convertInt8ToString(event.PreemptedComm[:])),
		event.IsPreempt,
		sanitizeString(convertInt8ToString(event.Comm[:])),
	)
	if err != nil {
		log.Errorf("failed to append to batch: %v", err)
		return batch, count, err
	}

	count++
	// 使用计数器替代 RowsWritten()
	if count >= 10 {
		if err := batch.Send(); err != nil {
			log.Errorf("failed to send batch: %v", err)
			return batch, count, err
		}
		count = 0 // 重置计数器
		// 创建新的批次
		batch, err = conn.PrepareBatch(ctx, `
			INSERT INTO sched_latency (
				pid, tid, delay_ns, ts, 
				preempted_pid, preempted_comm, 
				is_preempt, comm
			)
		`)
		if err != nil {
			log.Errorf("failed to prepare new batch: %v", err)
			return batch, count, err
		}
	}

	return batch, count, nil
}
