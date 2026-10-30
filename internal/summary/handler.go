package summary

import (
	"cmp"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kaz/pprotein/internal/collect"
	"github.com/labstack/echo/v4"
)

type (
	Handler struct {
		client *client
	}

	section struct {
		path   func(id string) string
		column string
	}
)

var (
	listTypes    = []string{"httplog", "slowlog", "cpu", "pprof", "memo"}
	statusTypes  = []string{"httplog", "slowlog", "cpu", "pprof"}
	commitTypes  = []string{"httplog", "slowlog", "cpu"}
	sectionTypes = map[string]section{
		"httplog": {path: func(id string) string { return "/api/httplog/" + url.PathEscape(id) }, column: "Sum"},
		"slowlog": {path: func(id string) string { return "/api/slowlog/" + id }, column: "Sum(QueryTime)"},
		"cpu":     {path: func(id string) string { return "/api/cpu/data/" + id }},
	}

	cellReplacer    = strings.NewReplacer("|", `\|`, "\r\n", " ", "\n", " ", "\r", " ")
	oneLineReplacer = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ")
)

const defaultTop = 20

func NewHandler(baseURL string) *Handler {
	return &Handler{client: &client{baseURL: baseURL}}
}

func (h *Handler) RegisterHandlers(g *echo.Group) {
	g.GET("", h.getIndex)
	g.GET("/:groupId", h.getGroup)
}

func (h *Handler) listAll() ([]*collect.Entry, []string) {
	var entries []*collect.Entry
	var failures []string
	for _, typ := range listTypes {
		list, err := h.client.list(typ)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", typ, err))
			continue
		}
		entries = append(entries, list...)
	}
	return entries, failures
}

func (h *Handler) getIndex(c echo.Context) error {
	entries, failures := h.listAll()
	groups := buildGroups(entries, c.QueryParam("label"))

	b := &strings.Builder{}
	fmt.Fprintf(b, "# pprotein groups\n\n")
	writeFailures(b, failures)
	if len(failures) > 0 {
		fmt.Fprintf(b, "\n")
	}

	if len(groups) == 0 {
		fmt.Fprintf(b, "group なし\n")
		return markdown(c, b)
	}

	fmt.Fprintf(b, "| GroupId | 時刻 | Label | httplog | slowlog | cpu | pprof | memo | commit |\n")
	fmt.Fprintf(b, "| --- | --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, g := range groups {
		for _, label := range labelsOf(g.Entries, statusTypes) {
			cells := []string{g.ID, g.Time.Format(time.RFC3339), label}
			for _, typ := range listTypes {
				cells = append(cells, statuses(g.Entries, typ, label))
			}

			var hashes []string
			for _, e := range g.Entries {
				if e.Snapshot.Type != "memo" && e.Snapshot.Label == label {
					if hash := shortHash(e); hash != "" && !slices.Contains(hashes, hash) {
						hashes = append(hashes, hash)
					}
				}
			}
			commit := "-"
			if len(hashes) > 0 {
				commit = "`" + strings.Join(hashes, "`, `") + "`"
			}
			cells = append(cells, commit)

			for i := range cells {
				cells[i] = cellReplacer.Replace(cells[i])
			}
			fmt.Fprintf(b, "| %s |\n", strings.Join(cells, " | "))
		}
	}
	return markdown(c, b)
}

func (h *Handler) getGroup(c echo.Context) error {
	top := defaultTop
	if raw := c.QueryParam("top"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 1 {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid top: %q", raw))
		}
		top = v
	}

	entries, failures := h.listAll()
	groups := buildGroups(entries, c.QueryParam("label"))

	var g *group
	if id := c.Param("groupId"); id == "latest" {
		if len(groups) > 0 {
			g = groups[0]
		}
	} else {
		if i := slices.IndexFunc(groups, func(g *group) bool { return g.ID == id }); i >= 0 {
			g = groups[i]
		}
	}
	if g == nil {
		return echo.NewHTTPError(http.StatusNotFound, "group not found")
	}

	var snapshots, memos []*collect.Entry
	for _, e := range g.Entries {
		if e.Snapshot.Type == "memo" {
			memos = append(memos, e)
		} else {
			snapshots = append(snapshots, e)
		}
	}
	slices.SortStableFunc(snapshots, func(a, b *collect.Entry) int {
		if r := cmp.Compare(slices.Index(statusTypes, a.Snapshot.Type), slices.Index(statusTypes, b.Snapshot.Type)); r != 0 {
			return r
		}
		return cmp.Compare(a.Snapshot.Label, b.Snapshot.Label)
	})

	b := &strings.Builder{}
	fmt.Fprintf(b, "# pprotein summary %s\n\n", g.ID)
	fmt.Fprintf(b, "- 時刻: %s\n", g.Time.Format(time.RFC3339))
	h.writeCommits(b, snapshots)

	var missing []string
	for _, typ := range statusTypes {
		if !slices.ContainsFunc(snapshots, func(e *collect.Entry) bool { return e.Snapshot.Type == typ }) {
			missing = append(missing, typ)
		}
	}
	if len(missing) > 0 {
		fmt.Fprintf(b, "- 欠落: %s\n", strings.Join(missing, ", "))
	}
	writeFailures(b, failures)

	fmt.Fprintf(b, "\n## status\n\n")
	fmt.Fprintf(b, "| Type | Label | URL | Status | Message |\n")
	fmt.Fprintf(b, "| --- | --- | --- | --- | --- |\n")
	for _, e := range snapshots {
		msg := ""
		if e.Status != collect.StatusOk {
			msg = e.Message
		}
		cells := []string{e.Snapshot.Type, e.Snapshot.Label, e.Snapshot.URL, string(e.Status), msg}
		for i := range cells {
			cells[i] = cellReplacer.Replace(cells[i])
		}
		fmt.Fprintf(b, "| %s |\n", strings.Join(cells, " | "))
	}

	fmt.Fprintf(b, "\n## memo\n\n")
	if len(memos) == 0 {
		fmt.Fprintf(b, "(なし)\n")
	}
	for i, m := range memos {
		if i > 0 {
			fmt.Fprintf(b, "\n")
		}
		fmt.Fprintf(b, "### %s %s\n\n%s\n", m.Snapshot.Label, m.Snapshot.Datetime.Format(time.RFC3339), strings.TrimRight(m.Message, "\n"))
	}

	for _, e := range snapshots {
		sec, ok := sectionTypes[e.Snapshot.Type]
		if !ok {
			continue
		}
		fmt.Fprintf(b, "\n## %s %s %s\n\n", e.Snapshot.Type, e.Snapshot.Label, e.Snapshot.URL)
		h.writeSection(b, e, sec, top)
	}

	return markdown(c, b)
}

func (h *Handler) writeCommits(b *strings.Builder, snapshots []*collect.Entry) {
	fmt.Fprintf(b, "- commit\n")

	var all []string
	for _, label := range labelsOf(snapshots, commitTypes) {
		var hashes, parts []string
		for _, e := range snapshots {
			if !slices.Contains(commitTypes, e.Snapshot.Type) || e.Snapshot.Label != label || e.Snapshot.Repository == nil {
				continue
			}
			hash := e.Snapshot.Repository.Hash
			if hash == "" || slices.Contains(hashes, hash) {
				continue
			}
			hashes = append(hashes, hash)
			if !slices.Contains(all, hash) {
				all = append(all, hash)
			}

			part := "`" + shortHash(e) + "`"
			if first, _, _ := strings.Cut(e.Snapshot.Repository.Message, "\n"); first != "" {
				part += " " + first
			}
			parts = append(parts, part)
		}

		info := "不明"
		if len(parts) > 0 {
			info = strings.Join(parts, ", ")
		}
		fmt.Fprintf(b, "  - %s: %s\n", label, info)
	}

	if len(all) >= 2 {
		fmt.Fprintf(b, "- 警告: Label 間で commit が異なる\n")
	}
	fmt.Fprintf(b, "- 注記: pprof の snapshot には commit が入らない\n")
}

func (h *Handler) writeSection(b *strings.Builder, e *collect.Entry, sec section, top int) {
	if e.Status != collect.StatusOk {
		fmt.Fprintf(b, "status: %s: %s\n", e.Status, oneLineReplacer.Replace(e.Message))
		return
	}

	text, err := h.client.text(sec.path(e.Snapshot.ID))
	if err != nil {
		fmt.Fprintf(b, "取得に失敗: %s\n", oneLineReplacer.Replace(err.Error()))
		return
	}

	if e.Snapshot.Type == "cpu" {
		mpstat, pidstat := summarizeCPU(text, top)
		fmt.Fprintf(b, "### mpstat Average\n\n")
		writeBlock(b, mpstat)
		fmt.Fprintf(b, "\n### pidstat\n\n")
		writeBlock(b, pidstat)
		return
	}

	out, total, sorted := topTSV(text, sec.column, top)
	shown := min(top, total)
	if sorted {
		fmt.Fprintf(b, "全 %d 行中 上位 %d 行\n\n", total, shown)
	} else {
		fmt.Fprintf(b, "並べ替え列 %s がないため元の順で上位 %d 行\n\n", sec.column, shown)
	}
	fmt.Fprintf(b, "```tsv\n%s```\n", out)
}

func writeBlock(b *strings.Builder, content string) {
	if content == "" {
		fmt.Fprintf(b, "(見つからない)\n")
		return
	}
	fmt.Fprintf(b, "```\n%s```\n", content)
}

func writeFailures(b *strings.Builder, failures []string) {
	for _, f := range failures {
		fmt.Fprintf(b, "- 一覧の取得に失敗: %s\n", oneLineReplacer.Replace(f))
	}
}

func labelsOf(entries []*collect.Entry, types []string) []string {
	var labels []string
	for _, e := range entries {
		if slices.Contains(types, e.Snapshot.Type) && !slices.Contains(labels, e.Snapshot.Label) {
			labels = append(labels, e.Snapshot.Label)
		}
	}
	slices.Sort(labels)
	return labels
}

func statuses(entries []*collect.Entry, typ string, label string) string {
	var r []string
	for _, e := range entries {
		if e.Snapshot.Type == typ && e.Snapshot.Label == label {
			r = append(r, string(e.Status))
		}
	}
	if len(r) == 0 {
		return "-"
	}
	return strings.Join(r, ", ")
}

func shortHash(e *collect.Entry) string {
	if e.Snapshot.Repository == nil {
		return ""
	}
	hash := e.Snapshot.Repository.Hash
	return hash[:min(7, len(hash))]
}

func markdown(c echo.Context, b *strings.Builder) error {
	return c.Blob(http.StatusOK, "text/markdown; charset=utf-8", []byte(b.String()))
}
