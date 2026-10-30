package summary

import (
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/goccy/go-json"
	"github.com/kaz/pprotein/internal/collect"
	"github.com/labstack/echo/v4"
)

func newMockAPI(t *testing.T, entries map[string][]*collect.Entry, data map[string]string, failTypes ...string) (*httptest.Server, *sync.Map) {
	t.Helper()
	called := &sync.Map{}
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		called.Store(r.URL.Path, true)
		if r.Method != http.MethodGet || r.URL.Path == "/api/group/collect" {
			t.Errorf("must not be called: %s", r.URL.Path)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		for _, prefix := range []string{"/api/httplog/", "/api/slowlog/", "/api/cpu/data/"} {
			if id, ok := strings.CutPrefix(r.URL.Path, prefix); ok {
				body, ok := data[id]
				if !ok {
					w.WriteHeader(http.StatusInternalServerError)
					io.WriteString(w, "no data")
					return
				}
				io.WriteString(w, body)
				return
			}
		}

		typ, ok := strings.CutPrefix(r.URL.Path, "/api/")
		if !ok || strings.Contains(typ, "/") {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if slices.Contains(failTypes, typ) {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		list := entries[typ]
		if list == nil {
			list = []*collect.Entry{}
		}
		buf, err := json.Marshal(list)
		if err != nil {
			t.Errorf("failed to marshal: %v", err)
		}
		w.Write(buf)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, called
}

func newSummaryEcho(srv *httptest.Server) *echo.Echo {
	e := echo.New()
	NewHandler(srv.URL).RegisterHandlers(e.Group("/api/summary"))
	return e
}

func get(t *testing.T, e *echo.Echo, path string) (int, string, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String(), rec.Header().Get(echo.HeaderContentType)
}

func groupsFixture() map[string][]*collect.Entry {
	return map[string][]*collect.Entry{
		"httplog": {
			newEntry("httplog", "h1", "g1", "app1", "http://a/log", at(10, 0), collect.StatusOk, "", "aaaaaaa1111"),
			newEntry("httplog", "h2", "g2", "app1", "http://a/log", at(11, 0), collect.StatusOk, "", ""),
			newEntry("httplog", "h3", "", "app1", "http://a/log", at(12, 0), collect.StatusOk, "", ""),
		},
		"pprof": {
			newEntry("pprof", "p1", "g2", "app1", "http://a/pprof", at(11, 0), collect.StatusOk, "", ""),
			newEntry("pprof", "p2", "g2", "app1", "http://a/pprof", at(11, 1), collect.StatusPending, "Processing", ""),
		},
	}
}

func mustContain(t *testing.T, body string, subs ...string) {
	t.Helper()
	for _, s := range subs {
		if !strings.Contains(body, s) {
			t.Errorf("body does not contain %q:\n%s", s, body)
		}
	}
}

func mustNotContain(t *testing.T, body string, subs ...string) {
	t.Helper()
	for _, s := range subs {
		if strings.Contains(body, s) {
			t.Errorf("body contains %q:\n%s", s, body)
		}
	}
}

func TestIndexListsGroupsNewestFirst(t *testing.T) {
	srv, _ := newMockAPI(t, groupsFixture(), nil)
	code, body, ct := get(t, newSummaryEcho(srv), "/api/summary")

	if code != http.StatusOK {
		t.Fatalf("code = %d: %s", code, body)
	}
	if ct != "text/markdown; charset=utf-8" {
		t.Errorf("content-type = %q", ct)
	}
	g2 := "| g2 | 2026-10-30T11:00:00Z | app1 | ok | - | - | ok, pending | - | - |"
	g1 := "| g1 | 2026-10-30T10:00:00Z | app1 | ok | - | - | - | - | `aaaaaaa` |"
	mustContain(t, body, g2, g1)
	if strings.Index(body, g2) > strings.Index(body, g1) {
		t.Errorf("g2 must come before g1:\n%s", body)
	}
	mustNotContain(t, body, "| 2026-10-30T12:00:00Z")
}

func TestIndexLabelFilterAndEmpty(t *testing.T) {
	entries := map[string][]*collect.Entry{
		"httplog": {
			newEntry("httplog", "h1", "g1", "app1", "http://a/log", at(10, 0), collect.StatusOk, "", ""),
			newEntry("httplog", "h2", "g1", "app2", "http://b/log", at(10, 0), collect.StatusOk, "", ""),
		},
		"memo": {
			newEntry("memo", "m1", "g1", "", "", at(10, 0), collect.StatusOk, "", ""),
		},
	}
	srv, _ := newMockAPI(t, entries, nil)
	e := newSummaryEcho(srv)

	_, body, _ := get(t, e, "/api/summary")
	mustContain(t, body,
		"| g1 | 2026-10-30T10:00:00Z | app1 | ok | - | - | - | 1 | - |",
		"| g1 | 2026-10-30T10:00:00Z | app2 | ok | - | - | - | 1 | - |",
	)

	_, body, _ = get(t, e, "/api/summary?label=app2")
	mustContain(t, body, "| g1 | 2026-10-30T10:00:00Z | app2 | ok | - | - | - | 1 | - |")
	mustNotContain(t, body, "| app1 |")

	empty, _ := newMockAPI(t, nil, nil)
	code, body, _ := get(t, newSummaryEcho(empty), "/api/summary")
	if code != http.StatusOK {
		t.Errorf("code = %d", code)
	}
	mustContain(t, body, "group なし")
}

func TestIndexListFailureIsNoted(t *testing.T) {
	srv, _ := newMockAPI(t, groupsFixture(), nil, "pprof")
	code, body, _ := get(t, newSummaryEcho(srv), "/api/summary")
	if code != http.StatusOK {
		t.Fatalf("code = %d: %s", code, body)
	}
	mustContain(t, body, "- 一覧の取得に失敗: pprof:")
}

func TestGroupLatestIgnoresEmptyGroupID(t *testing.T) {
	srv, _ := newMockAPI(t, groupsFixture(), map[string]string{"h2": "Count\tUri\tSum\n"})
	code, body, _ := get(t, newSummaryEcho(srv), "/api/summary/latest")
	if code != http.StatusOK {
		t.Fatalf("code = %d: %s", code, body)
	}
	if !strings.HasPrefix(body, "# pprotein summary g2\n") {
		t.Errorf("body = %s", body)
	}
}

func TestGroupNotFound(t *testing.T) {
	srv, _ := newMockAPI(t, groupsFixture(), map[string]string{"h1": "Count\tUri\tSum\n"})
	e := newSummaryEcho(srv)
	if code, _, _ := get(t, e, "/api/summary/nope"); code != http.StatusNotFound {
		t.Errorf("nope: code = %d", code)
	}
	if code, _, _ := get(t, e, "/api/summary/g1?label=nolabel"); code != http.StatusNotFound {
		t.Errorf("nolabel: code = %d", code)
	}

	empty, _ := newMockAPI(t, nil, nil)
	if code, _, _ := get(t, newSummaryEcho(empty), "/api/summary/latest"); code != http.StatusNotFound {
		t.Errorf("latest: code = %d", code)
	}
}

func TestGroupInvalidTop(t *testing.T) {
	srv, _ := newMockAPI(t, groupsFixture(), nil)
	e := newSummaryEcho(srv)
	for _, q := range []string{"top=0", "top=x"} {
		if code, _, _ := get(t, e, "/api/summary/g1?"+q); code != http.StatusBadRequest {
			t.Errorf("%s: code = %d", q, code)
		}
	}
}

func TestGroupPartialStatuses(t *testing.T) {
	entries := map[string][]*collect.Entry{
		"httplog": {newEntry("httplog", "h1", "g1", "app1", "http://a/log", at(10, 0), collect.StatusOk, "Ready", "")},
		"cpu":     {newEntry("cpu", "c1", "g1", "app1", "http://a/cpu", at(10, 0), collect.StatusFail, "a|b\nc", "")},
		"pprof":   {newEntry("pprof", "p1", "g1", "app1", "http://a/pprof", at(10, 0), collect.StatusPending, "Processing", "")},
	}
	srv, called := newMockAPI(t, entries, map[string]string{"h1": "Count\tUri\tSum\n1\t/a\t1\n"})
	code, body, _ := get(t, newSummaryEcho(srv), "/api/summary/g1")
	if code != http.StatusOK {
		t.Fatalf("code = %d: %s", code, body)
	}
	mustContain(t, body,
		"- 欠落: slowlog",
		"| cpu | app1 | http://a/cpu | fail | a\\|b c |",
		"| pprof | app1 | http://a/pprof | pending | Processing |",
		"| httplog | app1 | http://a/log | ok |  |",
		"status: fail: a|b",
	)
	if _, ok := called.Load("/api/cpu/data/c1"); ok {
		t.Errorf("/api/cpu/data/c1 must not be called")
	}
	if _, ok := called.Load("/api/httplog/h1"); !ok {
		t.Errorf("/api/httplog/h1 must be called")
	}
}

func TestGroupHttplogTopAndSort(t *testing.T) {
	entries := map[string][]*collect.Entry{
		"httplog": {newEntry("httplog", "h1", "g1", "app1", "http://a/log", at(10, 0), collect.StatusOk, "", "")},
		"slowlog": {newEntry("slowlog", "s1", "g1", "db", "http://a/slow", at(10, 0), collect.StatusOk, "", "")},
	}
	data := map[string]string{
		"h1": "Count\tUri\tSum\n1\t/low\t1\n1\t/high\t9\n1\t/mid\t5\n",
		"s1": "Count\tQuery\tSum(QueryTime)\n1\tSELECT low\t0.1\n1\tSELECT high\t3.5\n1\tSELECT mid\t1.2\n",
	}
	srv, _ := newMockAPI(t, entries, data)
	_, body, _ := get(t, newSummaryEcho(srv), "/api/summary/g1?top=2")

	httplog, slowlog, _ := strings.Cut(body, "## slowlog")
	mustContain(t, httplog, "全 3 行中 上位 2 行", "/high", "/mid")
	mustNotContain(t, httplog, "/low")
	mustContain(t, slowlog, "全 3 行中 上位 2 行", "SELECT high", "SELECT mid")
	mustNotContain(t, slowlog, "SELECT low")
}

func TestGroupSameLabelSameTypeAllSections(t *testing.T) {
	entries := map[string][]*collect.Entry{
		"httplog": {
			newEntry("httplog", "h1", "g1", "app1", "http://a/log1", at(10, 0), collect.StatusOk, "", ""),
			newEntry("httplog", "h2", "g1", "app1", "http://a/log2", at(10, 1), collect.StatusOk, "", ""),
		},
	}
	data := map[string]string{"h1": "Count\tUri\tSum\n", "h2": "Count\tUri\tSum\n"}
	srv, _ := newMockAPI(t, entries, data)
	_, body, _ := get(t, newSummaryEcho(srv), "/api/summary/g1")
	mustContain(t, body, "## httplog app1 http://a/log1", "## httplog app1 http://a/log2")
}

func TestGroupCPUSection(t *testing.T) {
	entries := map[string][]*collect.Entry{
		"cpu": {newEntry("cpu", "c1", "g1", "app1", "http://a/cpu", at(10, 0), collect.StatusOk, "", "")},
	}
	srv, _ := newMockAPI(t, entries, map[string]string{"c1": cpuFixture})
	_, body, _ := get(t, newSummaryEcho(srv), "/api/summary/g1")
	mustContain(t, body, "### mpstat Average", "Average:     all", "### pidstat", "app")
}

func TestGroupDataFetchError(t *testing.T) {
	entries := map[string][]*collect.Entry{
		"httplog": {newEntry("httplog", "h1", "g1", "app1", "http://a/log", at(10, 0), collect.StatusOk, "", "")},
	}
	srv, _ := newMockAPI(t, entries, nil)
	code, body, _ := get(t, newSummaryEcho(srv), "/api/summary/g1")
	if code != http.StatusOK {
		t.Fatalf("code = %d: %s", code, body)
	}
	mustContain(t, body, "取得に失敗:")
}

func TestGroupCommitAndMemo(t *testing.T) {
	app1 := newEntry("httplog", "h1", "g1", "app1", "http://a/log", at(10, 0), collect.StatusOk, "", "aaaaaaa1111")
	app1.Snapshot.Repository.Message = "first line\nsecond"
	entries := map[string][]*collect.Entry{
		"httplog": {
			app1,
			newEntry("httplog", "h2", "g1", "app2", "http://b/log", at(10, 0), collect.StatusOk, "", "bbbbbbb2222"),
		},
		"memo": {newEntry("memo", "m1", "g1", "app1", "", at(10, 5), collect.StatusOk, "score: 12345", "")},
	}
	data := map[string]string{"h1": "Count\tUri\tSum\n", "h2": "Count\tUri\tSum\n"}
	srv, _ := newMockAPI(t, entries, data)
	_, body, _ := get(t, newSummaryEcho(srv), "/api/summary/g1")
	mustContain(t, body,
		"- app1: `aaaaaaa` first line",
		"- app2: `bbbbbbb` ",
		"- 警告: Label 間で commit が異なる",
		"- 注記: pprof の snapshot には commit が入らない",
		"score: 12345",
	)
	mustNotContain(t, body, "second")

	same := map[string][]*collect.Entry{
		"httplog": {
			newEntry("httplog", "h1", "g1", "app1", "http://a/log", at(10, 0), collect.StatusOk, "", "aaaaaaa1111"),
			newEntry("httplog", "h2", "g1", "app2", "http://b/log", at(10, 0), collect.StatusOk, "", "aaaaaaa1111"),
		},
	}
	srv, _ = newMockAPI(t, same, data)
	_, body, _ = get(t, newSummaryEcho(srv), "/api/summary/g1")
	mustNotContain(t, body, "警告")
}
