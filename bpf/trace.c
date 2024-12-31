// SPDX-License-Identifier: (GPL-2.0-only OR BSD-2-Clause)
/* Copyright Martynas Pumputis */
/* Copyright Authors of Cilium */

#include "vmlinux.h"
#include "vmlinux-x86.h"
#include "bpf/bpf_helpers.h"
#include "bpf/bpf_core_read.h"
#include "bpf/bpf_tracing.h"
#include "bpf/bpf_endian.h"
#include "bpf/bpf_ipv6.h"

extern int LINUX_KERNEL_VERSION __kconfig;

// 定义数据结构来存储调度延迟信息
struct sched_latency_t
{
    __u32 pid;               // 进程ID
    __u32 tid;               // 线程ID
    __u64 delay_ns;          // 调度延迟(纳秒)
    __u64 ts;                // 时间戳
    __u32 preempted_pid;     // 被抢占的进程ID
    char preempted_comm[16]; // 被抢占的进程名
    __u64 is_preempt;        // 是否抢占(0: 否, 1: 是)
    char comm[16];           // 进程名
} __attribute__((packed));

struct sched_latency_t *unused_sched_latency_t __attribute__((unused));

// 定义 ring buffer 用于传输数据到用户空间
struct
{
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
    __uint(max_entries, 256 * 1024);
} sched_events SEC(".maps");

// 用于临时存储唤醒时间的 hash map
struct
{
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 10240);
    __type(key, __u32);
    __type(value, __u64);
} wakeup_times SEC(".maps");

struct trace_event_raw_sched_wakeup
{
    /* common fields */
    __u16 common_type;         /* offset: 0, size: 2 */
    __u8 common_flags;         /* offset: 2, size: 1 */
    __u8 common_preempt_count; /* offset: 3, size: 1 */
    __s32 common_pid;          /* offset: 4, size: 4 */

    /* event specific fields */
    char comm[16];         /* offset: 8, size: 16 */
    __s32 pid;             /* offset: 24, size: 4 */
    __s32 prio;            /* offset: 28, size: 4 */
    __s32 target_cpu;      /* offset: 32, size: 4 */
} __attribute__((packed)); /* 确保结构体紧凑，没有额外的填充字节 */

// sched_wakeup 跟踪点处理函数
SEC("tp_btf/sched_wakeup")
int sched_wakeup(u64 *ctx)
{
    struct task_struct *task = (void *)ctx[0];
    u32 pid = task->pid;
    if (pid == 0)
    {
        return 0;
    }

    __u64 ts = bpf_ktime_get_ns();

    // 记录唤醒时间
    bpf_map_update_elem(&wakeup_times, &pid, &ts, BPF_ANY);
    return 0;
}

// sched_wakeup_new 跟踪点处理函数
SEC("tp_btf/sched_wakeup_new")
int sched_wakeup_new(u64 *ctx)
{
    struct task_struct *task = (void *)ctx[0];
    u32 pid = task->pid;
    if (pid == 0)
    {
        return 0;
    }

    __u64 ts = bpf_ktime_get_ns();

    // 记录唤醒时间
    bpf_map_update_elem(&wakeup_times, &pid, &ts, BPF_ANY);
    return 0;
}

// 定义流控相关的常量和map
#define SAMPLING_RATIO 100   // 采样率 1/100
#define THRESHOLD_NS 1000000 // 延迟阈值 1ms

// 用于记录每个 CPU 的最后一次采样时间
struct
{
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, 1);
    __type(key, u32);
    __type(value, u64);
} last_sample SEC(".maps");

#define TASK_RUNNING 0

u64 get_task_cgroup_id(struct task_struct *task)
{
    u64 cgroup_id = 0;
    struct css_set *cgroups;

    // 使用 BPF_CORE_READ 安全地读取
    cgroups = BPF_CORE_READ(task, cgroups);
    if (cgroups)
    {
        cgroup_id = BPF_CORE_READ(cgroups, dfl_cgrp, kn, id);
    }

    return cgroup_id;
}

// 使用 kprobe 替代 tracepoint
SEC("kprobe/__switch_to")
int BPF_KPROBE(kprobe_sched_switch, struct task_struct *prev)
{
    // 获取当前进程（即 next）的 task_struct
    struct task_struct *next = (struct task_struct *)bpf_get_current_task();
    if (!next)
        return 0;

    // 读取进程信息
    u32 prev_pid = BPF_CORE_READ(prev, pid);
    u32 prev_tgid = BPF_CORE_READ(prev, tgid);
    u32 next_pid = BPF_CORE_READ(next, pid);
    u32 next_tgid = BPF_CORE_READ(next, tgid);

    bpf_printk("prev_pid: %d, prev_tgid: %d, next_pid: %d, next_tgid: %d\n", prev_pid, prev_tgid, next_pid, next_tgid);

    return 0;
}

// sched_switch 跟踪点处理函数（添加流控）
SEC("tp_btf/sched_switch")
int sched_switch(u64 *ctx)
{
    struct task_struct *prev = (struct task_struct *)ctx[1];
    struct task_struct *next = (struct task_struct *)ctx[2];

    u32 prev_pid = BPF_CORE_READ(prev, pid);
    u32 prev_tgid = BPF_CORE_READ(prev, tgid);
    u32 next_pid = BPF_CORE_READ(next, pid);
    u32 next_tgid = BPF_CORE_READ(next, tgid);

    __u64 *wakeup_ts;
    __u64 now = bpf_ktime_get_ns();

    // 跳过内核线程 (PID = 0)
    if (prev_pid == 0 || next_pid == 0)
    {
        return 0;
    }

    // 查找进程的唤醒时间
    wakeup_ts = bpf_map_lookup_elem(&wakeup_times, &next_pid);
    if (!wakeup_ts)
        return 0;

    // 计算调度延迟
    __u64 delay = now - *wakeup_ts;

    // 流控逻辑开始
    __u32 key = 0;
    __u64 *last_ts = bpf_map_lookup_elem(&last_sample, &key);
    if (!last_ts)
        return 0;

    // 基于时间的流控
    if ((now - *last_ts) < THRESHOLD_NS)
    {
        // 如果距离上次采样时间太短，执行采样判断
        if (bpf_get_prandom_u32() % SAMPLING_RATIO != 0)
        {
            bpf_map_delete_elem(&wakeup_times, &next_pid);
            return 0;
        }
    }

    // 更新最后采样时间
    bpf_map_update_elem(&last_sample, &key, &now, BPF_ANY);

    // 延迟阈值过滤
    if (delay < THRESHOLD_NS)
    {
        bpf_map_delete_elem(&wakeup_times, &next_pid);
        return 0;
    }

    u64 prev_cgroup_id = get_task_cgroup_id(prev);
    u64 next_cgroup_id = get_task_cgroup_id(next);

    // 准备输出数据
    struct sched_latency_t latency =
        {
            .pid = next_tgid,
            .tid = next_pid,
            .delay_ns = delay,
            .ts = now,
        };

    bpf_probe_read_kernel_str(&latency.comm, sizeof(latency.comm), next->comm);

#if defined(__TARGET_ARCH_x86)
    __u32 state = BPF_CORE_READ(prev, __state);
#elif defined(__TARGET_ARCH_arm64)
    __u32 state = BPF_CORE_READ(prev, state);
#else
    __u32 state = BPF_CORE_READ(prev, __state);
#endif

    // 如果前一个状态是 TASK_RUNNING，则认为是抢占, 记录被抢占的进程ID
    if (state == TASK_RUNNING)
    {
        latency.is_preempt = 1;
        latency.preempted_pid = prev_tgid;
        bpf_probe_read_kernel_str(&latency.preempted_comm, sizeof(latency.preempted_comm), prev->comm);
    }

    bpf_printk("pid: %d, tid: %d, delay: %llu ns, ts: %llu ns, comm: %s, is_preempt: %d, preempted_pid: %d, preempted_comm: %s, prev_cgroup_id: %llu, next_cgroup_id: %llu\n",
               latency.pid, latency.tid, latency.delay_ns, latency.ts, latency.comm, latency.is_preempt, latency.preempted_pid, latency.preempted_comm, prev_cgroup_id, next_cgroup_id);

    // 输出到 perf event
    bpf_perf_event_output(ctx, &sched_events, BPF_F_CURRENT_CPU, &latency, sizeof(latency));

    // 删除已处理的唤醒时间记录
    bpf_map_delete_elem(&wakeup_times, &next_pid);

    return 0;
}

char __license[] SEC("license") = "Dual BSD/GPL";
