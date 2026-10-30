package summary

import (
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/google/pprof/profile"
)

func readProfile(t *testing.T, path string) *profile.Profile {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	p, err := profile.Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func withoutTime(s string) string {
	var out []string
	for _, l := range strings.SplitAfter(s, "\n") {
		if !strings.HasPrefix(l, "Time:") {
			out = append(out, l)
		}
	}
	return strings.Join(out, "")
}

func TestTopArgs(t *testing.T) {
	got := topArgs("p1")
	want := []string{"-top", "-output", "summary-top", "p1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("topArgs = %q, want %q", got, want)
	}
}

func TestRunTopMatchesGoToolPprof(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	out, err := runTop(readProfile(t, "testdata/cpu.pprof"), "p1")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("testdata/cpu.top.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got := withoutTime(out); got != string(want) {
		t.Errorf("runTop output mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("HOME に %d 個のファイルが作られた", len(entries))
	}

	again, err := runTop(readProfile(t, "testdata/cpu.pprof"), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if again != out {
		t.Errorf("2 回目の出力が異なる\nfirst:\n%s\nsecond:\n%s", out, again)
	}
}

const topFixture = `File: app
Type: cpu
Time: 2026-10-30 10:00:00 UTC
Duration: 30s, Total samples = 3.10s (10.33%)
Showing nodes accounting for 3.10s, 100% of 3.10s total
      flat  flat%   sum%        cum   cum%
     1.50s 48.39% 48.39%      1.50s 48.39%  main.a
     0.90s 29.03% 77.42%      2.40s 77.42%  main.b
     0.50s 16.13% 93.55%      900ms 29.03%  main.c (inline)
     0.20s  6.45%   100%      2.50s 80.65%  main.(*T).d
`

func rowName(row string) string {
	return strings.Fields(row)[5]
}

func TestParseTop(t *testing.T) {
	r, err := parseTop(topFixture)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.header) != 5 || r.header[3] != "Duration: 30s, Total samples = 3.10s (10.33%)" {
		t.Errorf("header = %q", r.header)
	}
	if got := strings.Fields(r.columns); !reflect.DeepEqual(got, []string{"flat", "flat%", "sum%", "cum", "cum%"}) {
		t.Errorf("columns = %q", r.columns)
	}
	if len(r.rows) != 4 || !strings.Contains(r.rows[0], "main.a") {
		t.Fatalf("rows = %q", r.rows)
	}
	var names []string
	for _, row := range r.byCum() {
		names = append(names, rowName(row))
	}
	if want := []string{"main.(*T).d", "main.b", "main.a", "main.c"}; !reflect.DeepEqual(names, want) {
		t.Errorf("byCum = %q, want %q", names, want)
	}
	if !strings.Contains(r.rows[0], "main.a") || !strings.Contains(r.rows[3], "main.(*T).d") {
		t.Errorf("rows reordered: %q", r.rows)
	}
}

func TestParseTopNoColumns(t *testing.T) {
	if _, err := parseTop("Type: cpu\n"); err == nil {
		t.Error("error expected")
	}
}

func TestCumValueUnits(t *testing.T) {
	row := func(cum string) string {
		return fmt.Sprintf("%10s %s %s %10s %s  %s", "0", "0%", "0%", cum, "0%", "main.x")
	}
	for _, c := range [][2]string{
		{"1.50s", "900ms"}, {"2hrs", "59.50s"}, {"1ms", "999us"}, {"1us", "999ns"},
		{"1.50MB", "1000kB"}, {"1GB", "1023.50MB"}, {"12", "9"}, {"1ns", "0"},
	} {
		if a, b := cumValue(row(c[0])), cumValue(row(c[1])); !(a > b) {
			t.Errorf("cumValue(%s)=%v should be > cumValue(%s)=%v", c[0], a, c[1], b)
		}
	}
}

func TestByCumOnRealProfile(t *testing.T) {
	out, err := runTop(readProfile(t, "testdata/cpu.pprof"), "p1")
	if err != nil {
		t.Fatal(err)
	}
	r, err := parseTop(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.rows) == 0 {
		t.Fatal("no rows")
	}
	cum := r.byCum()
	if !reflect.DeepEqual(slices.Sorted(slices.Values(cum)), slices.Sorted(slices.Values(r.rows))) {
		t.Error("node set differs")
	}
	for i := 1; i < len(cum); i++ {
		if cumValue(cum[i-1]) < cumValue(cum[i]) {
			t.Errorf("not sorted at %d: %q then %q", i, cum[i-1], cum[i])
		}
	}
	want, err := os.ReadFile("testdata/cpu.top.txt")
	if err != nil {
		t.Fatal(err)
	}
	find := func(lines []string) string {
		for _, l := range lines {
			if strings.HasPrefix(l, "Duration:") {
				return l
			}
		}
		return ""
	}
	d := find(r.header)
	if d == "" || d != find(strings.Split(string(want), "\n")) {
		t.Errorf("Duration line mismatch: %q", d)
	}
}
