package summary

import (
	"os"
	"reflect"
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
