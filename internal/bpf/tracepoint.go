// SPDX-License-Identifier: Apache-2.0
/* Copyright 2024 Authors of Cilium */

package bpf

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"golang.org/x/sync/errgroup"
)

type tracing struct {
	sync.Mutex
	links []link.Link
	progs []*ebpf.Program
}

func (t *tracing) HaveTracing() bool {
	t.Lock()
	defer t.Unlock()

	return len(t.links) > 0
}

func (t *tracing) Detach() {
	t.Lock()
	defer t.Unlock()

	t.detach()

	for _, p := range t.progs {
		_ = p.Close()
	}
	t.progs = nil
}

func (t *tracing) detach() {
	var errg errgroup.Group

	for _, l := range t.links {
		l := l
		errg.Go(func() error {
			_ = l.Close()
			return nil
		})
	}

	_ = errg.Wait()
}

func (t *tracing) addLink(l link.Link) {
	t.Lock()
	defer t.Unlock()

	t.links = append(t.links, l)
}

func (t *tracing) Merge(adds ...*tracing) {
	t.Lock()
	defer t.Unlock()

	for _, add := range adds {
		t.links = append(t.links, add.links...)
		t.progs = append(t.progs, add.progs...)
	}
}

func (t *tracing) trace(group, tracingName string, prog *ebpf.Program) error {
	tracing, err := link.Tracepoint(group, tracingName, prog, nil)
	if err != nil {
		return fmt.Errorf("failed to attach tracing: %w", err)
	}

	t.addLink(tracing)

	return nil
}

func (t *tracing) Tracepoint(group string, progs map[string]*ebpf.Program) error {
	log.Printf("正在附加 %s 组的 TracePoint 程序...\n", group)

	for tracepointName, prog := range progs {
		if err := t.trace(group, tracepointName, prog); err != nil {
			return fmt.Errorf("附加 %s 组的 TracePoint 程序失败: %v", group, err)
		}
	}

	return nil
}

func (t *tracing) Tracing(group string, progs map[string]*ebpf.Program) error {
	log.Printf("正在附加 %s 组的 tp_btf 程序...\n", group)

	for _, prog := range progs {
		tracing, err := link.AttachTracing(link.TracingOptions{Program: prog})
		if err != nil {
			return fmt.Errorf("附加 %s 组的 TracePoint 程序失败: %v", group, err)
		}

		t.addLink(tracing)
	}

	return nil
}

func IsTracepointExist(group, tracepointName string) bool {
	tracepointPath := filepath.Join("/sys/kernel/debug/tracing/events", group, tracepointName, "enable")
	_, err := os.Stat(tracepointPath)
	return err == nil
}
