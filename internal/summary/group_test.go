package summary

import (
	"testing"
	"time"

	"github.com/kaz/pprotein/internal/collect"
	"github.com/kaz/pprotein/internal/git"
)

func newEntry(typ, id, groupID, label, url string, at time.Time, status collect.Status, msg, hash string) *collect.Entry {
	e := &collect.Entry{
		Snapshot: &collect.Snapshot{
			SnapshotMeta:   &collect.SnapshotMeta{ID: id, Type: typ, Datetime: at},
			SnapshotTarget: &collect.SnapshotTarget{GroupId: groupID, Label: label, URL: url},
		},
		Status:  status,
		Message: msg,
	}
	if hash != "" {
		e.Snapshot.Repository = &git.RepositoryInfo{Hash: hash, Message: "msg of " + hash}
	}
	return e
}

func at(h, m int) time.Time {
	return time.Date(2026, 10, 30, h, m, 0, 0, time.UTC)
}

func ids(g *group) []string {
	var r []string
	for _, e := range g.Entries {
		r = append(r, e.Snapshot.ID)
	}
	return r
}

func eqIDs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestBuildGroupsSkipsEmptyGroupID(t *testing.T) {
	entries := []*collect.Entry{
		newEntry("httplog", "a", "g1", "app1", "u", at(10, 0), collect.StatusOk, "", ""),
		newEntry("httplog", "c", "g2", "app1", "u", at(11, 1), collect.StatusOk, "", ""),
		newEntry("pprof", "b", "g2", "app1", "u", at(11, 0), collect.StatusOk, "", ""),
		newEntry("pprof", "x", "", "app1", "u", at(12, 0), collect.StatusOk, "", ""),
	}
	gs := buildGroups(entries, "")
	if len(gs) != 2 {
		t.Fatalf("len = %d", len(gs))
	}
	if gs[0].ID != "g2" || !gs[0].Time.Equal(at(11, 0)) || !eqIDs(ids(gs[0]), []string{"b", "c"}) {
		t.Errorf("g[0] = %+v %v", gs[0], ids(gs[0]))
	}
	if gs[1].ID != "g1" {
		t.Errorf("g[1] = %s", gs[1].ID)
	}
}

func TestBuildGroupsLabelFilter(t *testing.T) {
	entries := []*collect.Entry{
		newEntry("pprof", "a1", "g1", "app1", "u", at(10, 0), collect.StatusOk, "", ""),
		newEntry("pprof", "a2", "g1", "app2", "u", at(10, 1), collect.StatusOk, "", ""),
		newEntry("memo", "m", "g1", "", "u", at(10, 2), collect.StatusOk, "", ""),
		newEntry("pprof", "b1", "g2", "app1", "u", at(11, 0), collect.StatusOk, "", ""),
	}
	gs := buildGroups(entries, "app2")
	if len(gs) != 1 || gs[0].ID != "g1" {
		t.Fatalf("gs = %+v", gs)
	}
	if !eqIDs(ids(gs[0]), []string{"a2", "m"}) {
		t.Errorf("ids = %v", ids(gs[0]))
	}
}

func TestBuildGroupsMemoOnlyGroup(t *testing.T) {
	entries := []*collect.Entry{
		newEntry("pprof", "a", "g1", "app1", "u", at(10, 0), collect.StatusOk, "", ""),
		newEntry("memo", "m1", "g1", "", "u", at(9, 0), collect.StatusOk, "", ""),
		newEntry("memo", "m3", "g3", "", "u", at(12, 0), collect.StatusOk, "", ""),
	}
	gs := buildGroups(entries, "")
	if len(gs) != 1 || gs[0].ID != "g1" {
		t.Fatalf("gs = %+v", gs)
	}
	if !gs[0].Time.Equal(at(10, 0)) {
		t.Errorf("Time = %v", gs[0].Time)
	}
	if !eqIDs(ids(gs[0]), []string{"m1", "a"}) {
		t.Errorf("ids = %v", ids(gs[0]))
	}
}
