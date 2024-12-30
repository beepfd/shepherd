package bpf

import (
	"github.com/cilium/ebpf"
)

var (
	SchedTracepointProgs = map[string]string{
		"sched_wakeup":     "sched_wakeup",
		"sched_wakeup_new": "sched_wakeup_new",
		"sched_switch":     "sched_switch",
	}
)

func AttachSchedTracepoint(coll *ebpf.Collection) (*tracing, error) {
	schedTracepointProgs := map[string]*ebpf.Program{}
	for name, prog := range coll.Programs {
		key, ok := SchedTracepointProgs[name]
		if !ok {
			continue
		}

		schedTracepointProgs[key] = prog
	}

	trace := Tracing("sched", schedTracepointProgs)
	return trace, nil
}
