package cache

import "sync"

// SchedMetricsMap 保存pid和调度延迟的映射关系
// key: pid
// value: metadata.SchedMetrics
var SchedMetricsMap *sync.Map

// SchedPreemptedMap 保存pid和调度延迟的映射关系
// key: pid
// value: metadata.SchedPreempted
var SchedPreemptedMap *sync.Map

func init() {
	SchedMetricsMap = new(sync.Map)
	SchedPreemptedMap = new(sync.Map)
}
