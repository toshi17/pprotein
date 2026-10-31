package slp

import (
	"bytes"
	"strings"
	"testing"
)

func TestRewritePGExecuteLines(t *testing.T) {
	in := strings.Join([]string{
		"2026-10-31 01:50:00.100 UTC [3515] LOG:  duration: 1.234 ms  parse stmtcache_aaa: SELECT 1",
		"2026-10-31 01:50:00.102 UTC [3515] LOG:  duration: 5.678 ms  execute stmtcache_aaa: SELECT id FROM users WHERE id = $1",
		"2026-10-31 01:50:00.102 UTC [3515] DETAIL:  Parameters: $1 = '10'",
		"2026-10-31 01:50:00.300 UTC [3517] LOG:  duration: 2.000 ms  execute <unnamed>: SELECT p.id",
		"\tFROM posts p",
		"2026-10-31 01:50:00.400 UTC [3518] LOG:  duration: 4.000 ms  statement: UPDATE users SET name = 'execute x: y'",
	}, "\n")
	want := strings.Join([]string{
		"2026-10-31 01:50:00.100 UTC [3515] LOG:  duration: 1.234 ms  parse stmtcache_aaa: SELECT 1",
		"2026-10-31 01:50:00.102 UTC [3515] LOG:  duration: 5.678 ms  statement: SELECT id FROM users WHERE id = $1",
		"2026-10-31 01:50:00.102 UTC [3515] DETAIL:  Parameters: $1 = '10'",
		"2026-10-31 01:50:00.300 UTC [3517] LOG:  duration: 2.000 ms  statement: SELECT p.id",
		"\tFROM posts p",
		"2026-10-31 01:50:00.400 UTC [3518] LOG:  duration: 4.000 ms  statement: UPDATE users SET name = 'execute x: y'",
	}, "\n")

	var out bytes.Buffer
	if err := rewritePGExecuteLines(strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", out.String(), want)
	}
}
