package cpustat

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"sync"
)

type (
	CPUStatHandler struct{}

	command struct {
		name string
		args []string
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
		{"mpstat", []string{"-P", "ALL", sec, "1"}},
		{"pidstat", []string{"-u", "-h", sec, "1"}},
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
		fmt.Fprintf(w, "### %s\n\n%s\n", c.name, outputs[i])
	}
}

func run(ctx context.Context, c command) ([]byte, error) {
	cmd := exec.CommandContext(ctx, c.name, c.args...)
	cmd.Env = append(cmd.Environ(), "LC_ALL=C", "S_COLORS=never")
	return cmd.CombinedOutput()
}
