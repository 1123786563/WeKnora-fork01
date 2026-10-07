package modelcontext

import (
	"fmt"
	"strings"
	"testing"
)

func TestAnnotateGraphResultMarksTruncation(t *testing.T) {
	t.Parallel()
	base := "<retrieval mode=\"graph\">\n</retrieval>"
	rows := make([]map[string]interface{}, 0, 30)
	for i := 0; i < 30; i++ {
		rows = append(rows, map[string]interface{}{
			"source": "Docker", "type": fmt.Sprintf("rel-%02d", i), "target": "Kubernetes",
		})
	}
	got := annotateGraphResult(base, map[string]interface{}{
		"relations":         rows,
		"relations_total":   42,
		"relations_omitted": 12,
	})
	// The merged graphTruncationNote (upstream #3885 shape, fork keys) emits a
	// rich marker: attributes first, then the narrowing hint as text content.
	want := `<graph_truncated relations_shown="30" relations_total="42">`
	if !strings.Contains(got, want) {
		t.Fatalf("missing %q in %q", want, got)
	}
	if !strings.Contains(got, "</graph_truncated>") {
		t.Fatalf("marker must close after its hint text: %q", got)
	}
	if !strings.HasSuffix(got, "</retrieval>") {
		t.Fatalf("marker must stay inside the retrieval: %q", got)
	}
	// Without the truncation keys the list reads as complete.
	complete := annotateGraphResult(base, map[string]interface{}{"relations": rows[:2]})
	if strings.Contains(complete, "graph_truncated") {
		t.Fatalf("untruncated result must not carry a marker: %q", complete)
	}
}
