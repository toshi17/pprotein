package summary

import (
	"cmp"
	"slices"
	"time"

	"github.com/kaz/pprotein/internal/collect"
)

type group struct {
	ID      string
	Time    time.Time
	Entries []*collect.Entry
}

func buildGroups(entries []*collect.Entry, label string) []*group {
	byID := map[string]*group{}
	var memos []*collect.Entry

	for _, e := range entries {
		if e.Snapshot.GroupId == "" {
			continue
		}
		if e.Snapshot.Type == "memo" {
			memos = append(memos, e)
			continue
		}
		if label != "" && e.Snapshot.Label != label {
			continue
		}
		g, ok := byID[e.Snapshot.GroupId]
		if !ok {
			g = &group{ID: e.Snapshot.GroupId, Time: e.Snapshot.Datetime}
			byID[g.ID] = g
		}
		if e.Snapshot.Datetime.Before(g.Time) {
			g.Time = e.Snapshot.Datetime
		}
		g.Entries = append(g.Entries, e)
	}

	for _, m := range memos {
		if g, ok := byID[m.Snapshot.GroupId]; ok {
			g.Entries = append(g.Entries, m)
		}
	}

	groups := make([]*group, 0, len(byID))
	for _, g := range byID {
		slices.SortStableFunc(g.Entries, func(a, b *collect.Entry) int {
			return a.Snapshot.Datetime.Compare(b.Snapshot.Datetime)
		})
		groups = append(groups, g)
	}
	slices.SortFunc(groups, func(a, b *group) int {
		if c := b.Time.Compare(a.Time); c != 0 {
			return c
		}
		return cmp.Compare(b.ID, a.ID)
	})
	return groups
}
