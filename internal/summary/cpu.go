package summary

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
)

func summarizeCPU(text string, top int) (string, string) {
	return summarizeMpstat(cpuSection(text, "### mpstat")), summarizePidstat(cpuSection(text, "### pidstat"), top)
}

func cpuSection(text string, title string) []string {
	var section []string
	in := false
	for _, l := range strings.Split(text, "\n") {
		if strings.HasPrefix(l, "### ") {
			in = strings.TrimRight(l, " \r") == title
			continue
		}
		if in {
			section = append(section, l)
		}
	}
	return section
}

func summarizeMpstat(section []string) string {
	var out []string
	for _, l := range section {
		if strings.HasPrefix(l, "Average:") {
			out = append(out, l)
		}
	}
	if len(out) == 0 {
		return ""
	}
	return strings.Join(out, "\n") + "\n"
}

func summarizePidstat(section []string, top int) string {
	header := slices.IndexFunc(section, func(l string) bool { return strings.HasPrefix(l, "# ") })
	if header < 0 {
		return ""
	}

	var rows []string
	for _, l := range section[header+1:] {
		if strings.TrimSpace(l) != "" {
			rows = append(rows, l)
		}
	}

	if column := slices.Index(strings.Fields(strings.TrimPrefix(section[header], "# ")), "%CPU"); column >= 0 {
		cpu := func(l string) float64 {
			fields := strings.Fields(l)
			if column >= len(fields) {
				return 0
			}
			v, _ := strconv.ParseFloat(fields[column], 64)
			return v
		}
		slices.SortStableFunc(rows, func(a, b string) int { return cmp.Compare(cpu(b), cpu(a)) })
	}

	rows = rows[:min(top, len(rows))]
	return strings.Join(append([]string{section[header]}, rows...), "\n") + "\n"
}
