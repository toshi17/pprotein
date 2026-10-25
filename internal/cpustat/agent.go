package cpustat

import (
	"cmp"
	"context"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"
)

type (
	CPUStatHandler struct{}

	command struct {
		name   string
		args   []string
		format func([]byte) []byte
	}
)

func NewCPUStatHandler() *CPUStatHandler {
	return &CPUStatHandler{}
}

func (h *CPUStatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	seconds, err := strconv.Atoi(r.URL.Query().Get("seconds"))
	if err != nil || seconds <= 0 {
		seconds = 30
	}
	sec := strconv.Itoa(seconds)

	commands := []command{
		{"mpstat", []string{"-P", "ALL", sec, "1"}, nil},
		{"pidstat", []string{"-u", "-h", sec, "1"}, sortByCPU},
	}

	outputs := make([][]byte, len(commands))
	errs := make([]error, len(commands))

	var wg sync.WaitGroup
	for i, c := range commands {
		wg.Add(1)
		go func(i int, c command) {
			defer wg.Done()
			outputs[i], errs[i] = run(r.Context(), c)
		}(i, c)
	}
	wg.Wait()

	for i, c := range commands {
		if errs[i] != nil {
			log.Printf("failed to run %s: %v", c.name, errs[i])
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "failed to run %s: %v\n%s", c.name, errs[i], outputs[i])
			return
		}
	}

	for i, c := range commands {
		if c.format != nil {
			outputs[i] = c.format(outputs[i])
		}
		fmt.Fprintf(w, "### %s\n\n%s\n", c.name, outputs[i])
	}
}

func run(ctx context.Context, c command) ([]byte, error) {
	cmd := exec.CommandContext(ctx, c.name, c.args...)
	cmd.Env = append(cmd.Environ(), "LC_ALL=C", "S_COLORS=never")
	return cmd.CombinedOutput()
}

func sortByCPU(out []byte) []byte {
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")

	header := slices.IndexFunc(lines, func(l string) bool { return strings.HasPrefix(l, "# ") })
	if header < 0 {
		return out
	}
	column := slices.Index(strings.Fields(strings.TrimPrefix(lines[header], "# ")), "%CPU")
	if column < 0 {
		return out
	}

	cpu := func(l string) float64 {
		fields := strings.Fields(l)
		if column >= len(fields) {
			return 0
		}
		v, _ := strconv.ParseFloat(fields[column], 64)
		return v
	}
	rows := lines[header+1:]
	slices.SortStableFunc(rows, func(a, b string) int { return cmp.Compare(cpu(b), cpu(a)) })

	return []byte(strings.Join(lines, "\n") + "\n")
}
