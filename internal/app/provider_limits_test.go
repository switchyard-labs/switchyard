package app

import "testing"

func TestProviderLimitsFailClosedAndConfigurable(t *testing.T) {
	for _, name := range []string{"SWITCHYARD_AGENT_MEMORY_MIB", "SWITCHYARD_AGENT_PIDS", "SWITCHYARD_AGENT_CPU_PERCENT", "SWITCHYARD_AGENT_TIMEOUT_SECONDS", "SWITCHYARD_AGENT_OUTPUT_KIB"} {
		t.Setenv(name, "")
	}
	t.Setenv("SWITCHYARD_AGENT_MEMORY_MIB", "512")
	t.Setenv("SWITCHYARD_AGENT_PIDS", "64")
	limits, err := providerLimits()
	if err != nil || limits.MemoryBytes != 512<<20 || limits.PIDs != 64 || !limits.Cgroup {
		t.Fatalf("policy: %+v %v", limits, err)
	}
	for _, bad := range []string{"-1", "0", "unlimited", "999999999"} {
		t.Setenv("SWITCHYARD_AGENT_MEMORY_MIB", bad)
		if _, err := providerLimits(); err == nil {
			t.Fatal("accepted invalid memory policy")
		}
	}
}
