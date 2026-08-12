package httpx

import "runtime"

// stackTraceMaxBytes caps how much of the stack is logged after a panic. 16 KiB is enough to
// find the culprit without turning one panic into a megabyte of logs.
const stackTraceMaxBytes = 16 << 10

// stackTrace returns the current goroutine's stack, truncated to stackTraceMaxBytes.
func stackTrace() string {
	buf := make([]byte, stackTraceMaxBytes)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}
