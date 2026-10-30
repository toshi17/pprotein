package summary

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
)

func topTSV(tsv string, column string, top int) (string, int, bool) {
	var lines []string
	for _, l := range strings.Split(tsv, "\n") {
		if l != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) == 0 {
		return "", 0, false
	}

	header, rows := lines[0], lines[1:]
	total := len(rows)

	idx := slices.Index(strings.Split(header, "\t"), column)
	sorted := idx >= 0
	if sorted {
		value := func(row string) float64 {
			fields := strings.Split(row, "\t")
			if idx >= len(fields) {
				return 0
			}
			v, err := strconv.ParseFloat(strings.TrimSpace(fields[idx]), 64)
			if err != nil {
				return 0
			}
			return v
		}
		slices.SortStableFunc(rows, func(a, b string) int {
			return cmp.Compare(value(b), value(a))
		})
	}

	rows = rows[:min(top, len(rows))]
	return strings.Join(append([]string{header}, rows...), "\n") + "\n", total, sorted
}
