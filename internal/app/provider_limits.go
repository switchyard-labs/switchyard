package app

import (
	"fmt"
	"os"
	"strconv"
	"switchyard/internal/agent"
	"time"
)

func providerLimits() (agent.RunnerLimits, error) {
	var limits agent.RunnerLimits
	read := func(name string, fallback, min, max int) (int, error) {
		s := os.Getenv(name)
		if s == "" {
			return fallback, nil
		}
		n, err := strconv.Atoi(s)
		if err != nil || n < min || n > max {
			return 0, fmt.Errorf("invalid %s; expected %d–%d", name, min, max)
		}
		return n, nil
	}
	memory, err := read("SWITCHYARD_AGENT_MEMORY_MIB", 1024, 128, 4096)
	if err != nil {
		return limits, err
	}
	pids, err := read("SWITCHYARD_AGENT_PIDS", 128, 16, 1024)
	if err != nil {
		return limits, err
	}
	cpu, err := read("SWITCHYARD_AGENT_CPU_PERCENT", 200, 10, 800)
	if err != nil {
		return limits, err
	}
	wall, err := read("SWITCHYARD_AGENT_TIMEOUT_SECONDS", 120, 5, 600)
	if err != nil {
		return limits, err
	}
	output, err := read("SWITCHYARD_AGENT_OUTPUT_KIB", 1024, 16, 8192)
	if err != nil {
		return limits, err
	}
	limits = agent.RunnerLimits{Cgroup: true, MemoryBytes: uint64(memory) << 20, PIDs: pids, CPUQuotaPercent: cpu, Timeout: time.Duration(wall) * time.Second, OutputBytes: output << 10, CPUSeconds: 120}
	return limits, nil
}
