package agent

import "testing"

func TestResourceFailureUsesKernelEvents(t *testing.T) {
	for _, tc := range []struct {
		name  string
		usage map[string]string
		want  string
	}{
		{"large reservation", map[string]string{"VmSize": "135453956 kB", "memory.events": "max 0\noom 0\noom_kill 0"}, "runner_exit_nonzero"},
		{"memory kill", map[string]string{"memory.events": "max 7\noom 1\noom_kill 1"}, "runner_memory_limit"},
		{"pid denied", map[string]string{"pids.events": "max 1"}, "runner_pid_limit"},
		{"missing accounting", nil, "runner_exit_nonzero"},
		{"memory pressure without kill", map[string]string{"memory.events": "max 20\noom 0\noom_kill 0"}, "runner_exit_nonzero"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := resourceFailureCode(tc.usage); got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}
