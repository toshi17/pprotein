package slp

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"

	"github.com/kaz/pprotein/internal/collect"
)

type (
	processor struct {
		confPath string
		db       string
	}
)

// slp pg は "statement:" の行だけを集計するため、拡張プロトコルのドライバーが出す "execute <name>:" の行を書き換える
var pgExecuteRe = regexp.MustCompile(`(LOG:\s+duration:\s+[0-9.]+\s+ms\s+)execute [^:]*:`)

func (p *processor) Cacheable() bool {
	return true
}

func (p *processor) Process(snapshot *collect.Snapshot) (io.ReadCloser, error) {
	bodyPath, err := snapshot.BodyPath()
	if err != nil {
		return nil, fmt.Errorf("failed to find snapshot body: %w", err)
	}

	subcommand := "my"
	if p.db == "postgresql" {
		subcommand = "pg"
		rewritten, err := rewritePGExecute(bodyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to rewrite postgresql log: %w", err)
		}
		defer os.Remove(rewritten)
		bodyPath = rewritten
	}

	cmd := exec.Command("slp", subcommand, "--config", p.confPath, "--output", "standard", "--format", "tsv", "--file", bodyPath)

	res, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("external process aborted: %w", err)
	}

	return io.NopCloser(bytes.NewBuffer(res)), nil
}

func rewritePGExecute(path string) (string, error) {
	in, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer in.Close()

	out, err := os.CreateTemp("", "pprotein-slowlog-*.log")
	if err != nil {
		return "", err
	}
	defer out.Close()

	if err := rewritePGExecuteLines(in, out); err != nil {
		os.Remove(out.Name())
		return "", err
	}
	return out.Name(), nil
}

func rewritePGExecuteLines(r io.Reader, w io.Writer) error {
	br := bufio.NewReader(r)
	bw := bufio.NewWriter(w)
	for {
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			if _, werr := bw.WriteString(pgExecuteRe.ReplaceAllString(line, "${1}statement:")); werr != nil {
				return werr
			}
		}
		if err == io.EOF {
			return bw.Flush()
		}
		if err != nil {
			return err
		}
	}
}
