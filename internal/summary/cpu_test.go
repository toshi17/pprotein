package summary

import (
	"strings"
	"testing"
)

const cpuFixture = `### mpstat

Linux 6.1 (host) 	10/30/26 	_aarch64_	(2 CPU)

10:00:00     CPU    %usr   %nice    %sys %iowait    %irq   %soft  %steal  %guest  %gnice   %idle
10:00:30     all   50.00    0.00   10.00    0.00    0.00    0.00    0.00    0.00    0.00   40.00
Average:     CPU    %usr   %nice    %sys %iowait    %irq   %soft  %steal  %guest  %gnice   %idle
Average:     all   50.00    0.00   10.00    0.00    0.00    0.00    0.00    0.00    0.00   40.00

### pidstat

Linux 6.1 (host) 	10/30/26 	_aarch64_	(2 CPU)

# Time        UID       PID    %usr %system  %guest   %wait    %CPU   CPU  Command
 10:00:30     1000       10    1.00    0.00    0.00    0.00    1.00     0  low
 10:00:30     1000       20   80.00    5.00    0.00    0.00   85.00     1  app
 10:00:30     1000       30   10.00    0.00    0.00    0.00   10.00     0  mysqld
`

func TestSummarizeCPU(t *testing.T) {
	mpstat, pidstat := summarizeCPU(cpuFixture, 2)

	if got := strings.Count(mpstat, "\n"); got != 2 {
		t.Errorf("mpstat lines = %d, want 2: %q", got, mpstat)
	}
	if !strings.HasPrefix(mpstat, "Average:") || strings.Contains(mpstat, "10:00:30     all") {
		t.Errorf("unexpected mpstat: %q", mpstat)
	}

	lines := strings.Split(strings.TrimSuffix(pidstat, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("pidstat lines = %d, want 3: %q", len(lines), pidstat)
	}
	if !strings.HasPrefix(lines[0], "# Time") || !strings.Contains(lines[1], "app") || !strings.Contains(lines[2], "mysqld") {
		t.Errorf("unexpected pidstat: %q", pidstat)
	}
	if strings.Contains(pidstat, "low") {
		t.Errorf("pidstat contains low: %q", pidstat)
	}
}

func TestSummarizeCPUMissingSections(t *testing.T) {
	mpstat, pidstat := summarizeCPU("garbage\n", 20)
	if mpstat != "" || pidstat != "" {
		t.Errorf("got (%q, %q), want empty", mpstat, pidstat)
	}

	pidOnly := cpuFixture[strings.Index(cpuFixture, "### pidstat"):]
	mpstat, pidstat = summarizeCPU(pidOnly, 20)
	if mpstat != "" {
		t.Errorf("mpstat = %q, want empty", mpstat)
	}
	if pidstat == "" {
		t.Error("pidstat is empty")
	}
}
