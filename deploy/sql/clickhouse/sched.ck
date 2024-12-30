-- `default`.sched_latency definition

CREATE TABLE default.sched_latency
(

    `pid` UInt32,

    `tid` UInt32,

    `delay_ns` UInt64,

    `ts` UInt64,

    `preempted_pid` UInt32,

    `preempted_comm` String,

    `is_preempt` UInt8,

    `comm` String,

    `date` Date DEFAULT today(),

    `datetime` DateTime64(9) DEFAULT now64(9)
)
ENGINE = MergeTree
ORDER BY (date,
 ts)
SETTINGS index_granularity = 8192;