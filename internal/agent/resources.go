package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// watchCgroup captures kernel accounting while a transient user service exists.
// Accounting is sampled, so the last value may precede final process exit.
func watchCgroup(unit string) func() map[string]string {
	done := make(chan struct{})
	finished := make(chan map[string]string, 1)
	dir := filepath.Join("/sys/fs/cgroup/user.slice", fmt.Sprintf("user-%d.slice", os.Getuid()), fmt.Sprintf("user@%d.service", os.Getuid()), "app.slice", unit)
	go func() {
		values := map[string]string{"accounting": "sampled cgroup v2; last sample may precede exit"}
		tick := time.NewTicker(50 * time.Millisecond)
		defer tick.Stop()
		sample := func() {
			for _, name := range []string{"memory.current", "memory.peak", "memory.events", "memory.max", "memory.swap.max", "pids.current", "pids.peak", "pids.max", "pids.events", "cpu.stat", "cpu.max"} {
				data, err := os.ReadFile(filepath.Join(dir, name))
				if err != nil {
					continue
				}
				value := strings.TrimSpace(string(data))
				values[name] = value
				if name == "memory.current" || name == "pids.current" {
					n, _ := strconv.ParseUint(value, 10, 64)
					old, _ := strconv.ParseUint(values[name+".sampled_peak"], 10, 64)
					if n > old {
						values[name+".sampled_peak"] = value
					}
				}
			}
		}
		for {
			select {
			case <-tick.C:
				sample()
			case <-done:
				sample()
				finished <- values
				return
			}
		}
	}()
	return func() map[string]string { close(done); return <-finished }
}

func resourceCounter(text, key string) uint64 {
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == key {
			n, _ := strconv.ParseUint(fields[1], 10, 64)
			return n
		}
	}
	return 0
}
