package metadata

type SchedMetrics struct {
	Pid           uint32 // 进程ID
	DelayNs       uint64 // 调度延迟
	Ts            uint64 // 时间戳
	PreempteCount uint64 // 被抢占的次数
	Comm          string // 进程名
}

type SchedPreempted struct {
	Pid   uint32 // 被抢占的进程
	Count uint64 // 被抢占的次数
	Comm  string // 被抢占的进程名
}
