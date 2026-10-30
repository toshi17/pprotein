package summary

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/pprof/driver"
	"github.com/google/pprof/profile"
	"github.com/kaz/pprotein/internal/pprof"
)

const topOutputName = "summary-top"

type (
	topFetcher struct {
		p *profile.Profile
	}

	topWriter struct {
		buf *bytes.Buffer
	}

	nopWriteCloser struct {
		io.Writer
	}

	silentUI struct{}
)

func (f *topFetcher) Fetch(src string, duration, timeout time.Duration) (*profile.Profile, string, error) {
	return f.p, "", nil
}

func (w *topWriter) Open(name string) (io.WriteCloser, error) {
	if name != topOutputName {
		return nil, fmt.Errorf("unexpected output: %s", name)
	}
	return nopWriteCloser{w.buf}, nil
}

func (nopWriteCloser) Close() error {
	return nil
}

func (silentUI) ReadLine(prompt string) (string, error) {
	return "", io.EOF
}

func (silentUI) Print(args ...interface{}) {}

func (silentUI) PrintErr(args ...interface{}) {}

func (silentUI) IsTerminal() bool {
	return false
}

func (silentUI) WantBrowser() bool {
	return false
}

func (silentUI) SetAutoComplete(complete func(string) string) {}

func topArgs(source string) []string {
	return []string{"-top", "-output", topOutputName, source}
}

func runTop(p *profile.Profile, source string) (string, error) {
	w := &topWriter{&bytes.Buffer{}}
	options := &driver.Options{
		Flagset: pprof.NewFlagSet(topArgs(source)),
		Fetch:   &topFetcher{p},
		Writer:  w,
		UI:      silentUI{},
	}
	if err := driver.PProf(options); err != nil {
		return "", fmt.Errorf("pprof internal error: %w", err)
	}
	if w.buf.Len() == 0 {
		// -output は driver のプロセス全体の config に入るため、既存 processor の実行と重なると stdout に出て空になる
		return "", errors.New("pprof の出力が空")
	}
	return w.buf.String(), nil
}

type topReport struct {
	header  []string
	columns string
	rows    []string
}

func parseTop(out string) (*topReport, error) {
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		f := strings.Fields(line)
		if len(f) < 5 || !slices.Equal(f[:5], []string{"flat", "flat%", "sum%", "cum", "cum%"}) {
			continue
		}
		r := &topReport{header: lines[:i], columns: line}
		for _, row := range lines[i+1:] {
			if strings.TrimSpace(row) != "" {
				r.rows = append(r.rows, row)
			}
		}
		return r, nil
	}
	return nil, errors.New("pprof の出力に列見出しがない")
}

func (r *topReport) byCum() []string {
	rows := slices.Clone(r.rows)
	slices.SortStableFunc(rows, func(a, b string) int {
		return cmp.Compare(cumValue(b), cumValue(a))
	})
	return rows
}

var unitScales = map[string]float64{
	"ns": 1, "us": 1e3, "ms": 1e6, "s": 1e9, "hrs": 3600e9,
	"B": 1, "kB": 1 << 10, "MB": 1 << 20, "GB": 1 << 30, "TB": 1 << 40, "PB": 1 << 50,
	"n*GCU": 1e-9, "u*GCU": 1e-6, "m*GCU": 1e-3, "GCU": 1, "k*GCU": 1e3, "M*GCU": 1e6, "G*GCU": 1e9, "T*GCU": 1e12, "P*GCU": 1e15,
}

// cum% は 1% 未満が有効数字 2 桁で潰れるため、単位付きの cum 列を数値化して比べる
func cumValue(row string) float64 {
	f := strings.Fields(row)
	if len(f) < 4 {
		return 0
	}
	s := f[3]
	i := 0
	for i < len(s) && (s[i] == '.' || s[i] == '-' || s[i] == '+' || s[i] >= '0' && s[i] <= '9') {
		i++
	}
	v, err := strconv.ParseFloat(s[:i], 64)
	if err != nil {
		return 0
	}
	if scale, ok := unitScales[s[i:]]; ok {
		v *= scale
	}
	return v
}
