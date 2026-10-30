package summary

import "testing"

func TestTopTSVSortsBySumDesc(t *testing.T) {
	in := "Count\tUri\tSum\n1\t/a\t1.5\n2\t/b\t10.0\n3\t/c\t3\n"
	out, total, sorted := topTSV(in, "Sum", 2)
	if out != "Count\tUri\tSum\n2\t/b\t10.0\n3\t/c\t3\n" || total != 3 || !sorted {
		t.Errorf("got (%q, %d, %v)", out, total, sorted)
	}
}

func TestTopTSVNonNumericTreatedAsZero(t *testing.T) {
	in := "Uri\tSum\n/a\tx\n/b\t1\n"
	out, total, sorted := topTSV(in, "Sum", 5)
	if out != "Uri\tSum\n/b\t1\n/a\tx\n" || total != 2 || !sorted {
		t.Errorf("got (%q, %d, %v)", out, total, sorted)
	}
}

func TestTopTSVMissingColumnKeepsOrder(t *testing.T) {
	in := "Count\tUri\n1\t/a\n2\t/b\n3\t/c\n"
	out, total, sorted := topTSV(in, "Sum", 2)
	if out != "Count\tUri\n1\t/a\n2\t/b\n" || total != 3 || sorted {
		t.Errorf("got (%q, %d, %v)", out, total, sorted)
	}
}

func TestTopTSVEdgeInputs(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		top    int
		out    string
		total  int
		sorted bool
	}{
		{"empty", "", 5, "", 0, false},
		{"header only", "Count\tSum\n", 5, "Count\tSum\n", 0, true},
		{"trailing blank line", "Count\tSum\n1\t2\n\n", 100, "Count\tSum\n1\t2\n", 1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, total, sorted := topTSV(tt.in, "Sum", tt.top)
			if out != tt.out || total != tt.total || sorted != tt.sorted {
				t.Errorf("got (%q, %d, %v)", out, total, sorted)
			}
		})
	}
}
