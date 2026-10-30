package summary

import (
	"bytes"
	"errors"
	"fmt"
	"io"
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
