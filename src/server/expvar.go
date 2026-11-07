package server

import (
	"expvar"
	"runtime"
	"time"
)

var (
	debugRequestCount    = expvar.NewInt("requests_total")
	debugRequestDuration = expvar.NewFloat("requests_duration_seconds")
	debugErrorCount      = expvar.NewInt("errors_total")
	debugStartTime       = time.Now()
)

func init() {
	expvar.Publish("uptime_seconds", expvar.Func(func() interface{} {
		return time.Since(debugStartTime).Seconds()
	}))
	expvar.Publish("goroutines", expvar.Func(func() interface{} {
		return runtime.NumGoroutine()
	}))
	expvar.Publish("memory", expvar.Func(func() interface{} {
		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		return map[string]uint64{
			"alloc":       stats.Alloc,
			"total_alloc": stats.TotalAlloc,
			"sys":         stats.Sys,
			"heap_alloc":  stats.HeapAlloc,
			"heap_sys":    stats.HeapSys,
		}
	}))
}

func recordDebugRequest(duration time.Duration, status int) {
	debugRequestCount.Add(1)
	debugRequestDuration.Add(duration.Seconds())
	if status >= 500 {
		debugErrorCount.Add(1)
	}
}
