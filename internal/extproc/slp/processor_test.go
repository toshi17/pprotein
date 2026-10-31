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

func TestRewritePGExecuteLinesJoinsStatementStartingWithNewline(t *testing.T) {
	in := strings.Join([]string{
		"2026-10-31 02:05:24.421 UTC [16590] LOG:  duration: 2.150 ms  bind stmtcache_a: ",
		"\t   SELECT i.data",
		"2026-10-31 02:05:24.423 UTC [16590] LOG:  duration: 1.966 ms  execute stmtcache_a: ",
		"\t   SELECT i.data, i.media_type",
		"\t   FROM image_objects i",
		"2026-10-31 02:05:24.423 UTC [16590] DETAIL:  parameters: $1 = 'x'",
		"2026-10-31 02:05:24.500 UTC [16591] LOG:  duration: 3.000 ms  statement: ",
		"2026-10-31 02:05:24.600 UTC [16592] LOG:  duration: 4.000 ms  execute stmtcache_b: ",
		"\tUPDATE users SET name = $1",
	}, "\n")
	want := strings.Join([]string{
		"2026-10-31 02:05:24.421 UTC [16590] LOG:  duration: 2.150 ms  bind stmtcache_a: ",
		"\t   SELECT i.data",
		"2026-10-31 02:05:24.423 UTC [16590] LOG:  duration: 1.966 ms  statement: SELECT i.data, i.media_type",
		"\t   FROM image_objects i",
		"2026-10-31 02:05:24.423 UTC [16590] DETAIL:  parameters: $1 = 'x'",
		"2026-10-31 02:05:24.500 UTC [16591] LOG:  duration: 3.000 ms  statement: ",
		"2026-10-31 02:05:24.600 UTC [16592] LOG:  duration: 4.000 ms  statement: UPDATE users SET name = $1",
	}, "\n")

	var out bytes.Buffer
	if err := rewritePGExecuteLines(strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", out.String(), want)
	}
}
