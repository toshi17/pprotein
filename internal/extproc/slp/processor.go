package slp

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"

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

// slp pg は "statement:" の後ろが同じ行で空だとエントリを捨てるため、改行から始まる SQL は次の行を連結する
var pgEmptyStatementRe = regexp.MustCompile(`LOG:\s+duration:\s+[0-9.]+\s+ms\s+statement:[ \t]*\r?\n$`)

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
	pending := ""
	for {
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			line = pgExecuteRe.ReplaceAllString(line, "${1}statement:")
			if pending != "" {
				if strings.HasPrefix(line, "\t") || strings.HasPrefix(line, " ") {
					line = strings.TrimRight(pending, " \t\r\n") + " " + strings.TrimLeft(line, " \t")
				} else if _, werr := bw.WriteString(pending); werr != nil {
					return werr
				}
				pending = ""
			}
			if pgEmptyStatementRe.MatchString(line) {
				pending = line
			} else if _, werr := bw.WriteString(line); werr != nil {
				return werr
			}
		}
		if err == io.EOF {
			if _, werr := bw.WriteString(pending); werr != nil {
				return werr
			}
			return bw.Flush()
		}
		if err != nil {
			return err
		}
	}
}
