//go:generate sh -c "echo Generating for $TARGET_GOARCH"
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -type sched_latency_t -target $TARGET_GOARCH -go-package binary -output-dir ./internal/binary -cc clang -no-strip Shepherd ./bpf/trace.c -- -I./bpf/headers -Wno-address-of-packed-member

package main
