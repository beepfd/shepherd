package output

import (
	"bytes"
	"encoding/binary"
	"strings"

	"github.com/cilium/ebpf/perf"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/pkg/errors"
)

func parseEvent(rd *perf.Reader, data interface{}) error {
	record, err := rd.Read()
	if err != nil {
		return err
	}

	if record.RawSample == nil {
		return errors.New("record.RawSample is nil")
	}

	if err := binary.Read(bytes.NewBuffer(record.RawSample), binary.LittleEndian, data); err != nil {
		return err
	}

	return nil
}

func parseRingbufEvent(record *ringbuf.Record, data interface{}) error {
	if err := binary.Read(bytes.NewBuffer(record.RawSample), binary.LittleEndian, data); err != nil {
		return err
	}

	return nil
}

func convertInt8ToString(data []int8) string {
	var result strings.Builder
	for _, b := range data {
		if b == 0 {
			break
		}
		result.WriteByte(byte(b))
	}
	return result.String()
}

func parseFileName(bs []int8) string {
	ba := make([]byte, 0, len(bs))
	for _, b := range bs {
		ba = append(ba, byte(b))
	}
	return filterNonASCII(ba)
}

func filterNonASCII(data []byte) string {
	var sb strings.Builder
	for _, b := range data {
		if b >= 32 && b <= 126 { // 只保留可见 ASCII 字符
			sb.WriteByte(b)
		}
	}
	return sb.String()
}

func sanitizeString(s string) string {
	return strings.TrimSpace(s)
}


// 线程状态常量
const (
	TASK_RUNNING           = 0x00000000
	TASK_INTERRUPTIBLE     = 0x00000001
	TASK_UNINTERRUPTIBLE   = 0x00000002
	TASK_STOPPED           = 0x00000004
	TASK_TRACED            = 0x00000008
	EXIT_DEAD              = 0x00000010
	EXIT_ZOMBIE            = 0x00000020
	EXIT_TRACE             = EXIT_ZOMBIE | EXIT_DEAD
	TASK_PARKED            = 0x00000040
	TASK_DEAD              = 0x00000080
	TASK_WAKEKILL          = 0x00000100
	TASK_WAKING            = 0x00000200
	TASK_NOLOAD            = 0x00000400
	TASK_NEW               = 0x00000800
	TASK_RTLOCK_WAIT       = 0x00001000
	TASK_FREEZABLE         = 0x00002000
	TASK_FREEZABLE_UNSAFE  = 0x00004000 // 取决于: IS_ENABLED(CONFIG_LOCKDEP)
	TASK_FROZEN            = 0x00008000
	TASK_STATE_MAX         = 0x00010000 // 截至 Linux 内核 6.9
)

// 任务状态映射表
var taskStates = map[uint32]string{
	0x00000000: "R",  // "RUNNING"
	0x00000001: "S",  // "INTERRUPTIBLE"
	0x00000002: "D",  // "UNINTERRUPTIBLE"
	0x00000004: "T",  // "STOPPED"
	0x00000008: "t",  // "TRACED"
	0x00000010: "X",  // "EXIT_DEAD"
	0x00000020: "Z",  // "EXIT_ZOMBIE"
	0x00000040: "P",  // "PARKED"
	0x00000080: "dd", // "DEAD"
	0x00000100: "wk", // "WAKEKILL"
	0x00000200: "wg", // "WAKING"
	0x00000400: "I",  // "NOLOAD"
	0x00000800: "N",  // "NEW"
	0x00001000: "rt", // "RTLOCK_WAIT"
	0x00002000: "fe", // "FREEZABLE"
	0x00004000: "fu", // "__TASK_FREEZABLE_UNSAFE = (0x00004000 * IS_ENABLED(CONFIG_LOCKDEP))"
	0x00008000: "fo", // "FROZEN"
}

// GetTaskStateName 将内核任务状态位掩码转换为可读字符串
func GetTaskStateName(taskState uint32) string {
	if taskState == 0 {
		return "R"
	}
	if taskState&TASK_NOLOAD != 0 { // 空闲内核线程等待工作
		return "I"
	}

	var names []string
	for state, name := range taskStates {
		if taskState&state != 0 {
			names = append(names, name)
		}
	}

	return strings.Join(names, "+")
}