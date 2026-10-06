package neo4j

import (
	"context"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/neo4j/neo4j-go-driver/v6/neo4j"
)

// TestGraphSearchCypherBoundsAndOrders pins the three review points on #3550:
// relation-less seeds are excluded, exact name matches outrank substring-only
// hits, and both LIMITs are preceded by the same ordering key so a truncated
// result keeps the neighbourhoods of the highest-ranked seeds.
func TestGraphSearchCypherBoundsAndOrders(t *testing.T) {
	query, params := graphSearchCypher("ENTITY_kb", []string{"安恒"})

	for _, want := range []string{
		"EXISTS { (n)--() }",
		"CASE WHEN n.name IN $nodes THEN 0 ELSE 1 END AS seed_rank",
		"ORDER BY seed_rank, name_len, name",
		"LIMIT $maxSeedNodes",
		"LIMIT $maxRows",
	} {
		if !strings.Contains(query, want) {
			t.Errorf("query is missing %q:\n%s", want, query)
		}
	}

	// The ordering key has to reach both caps: a row LIMIT without it keeps an
	// arbitrary subset of the rows.
	if got := strings.Count(query, "ORDER BY seed_rank, name_len, name"); got != 2 {
		t.Errorf("ORDER BY applied %d time(s), want 2 (seed cap and row cap):\n%s", got, query)
	}
	if strings.Index(query, "LIMIT $maxSeedNodes") > strings.Index(query, "LIMIT $maxRows") {
		t.Errorf("the seed cap must be applied before the row cap:\n%s", query)
	}
	// A relation-less seed expands to nothing, so it must not survive the first
	// cap (the check has to sit in the first WHERE, before the seed ordering).
	whereAt := strings.Index(query, "EXISTS { (n)--() }")
	seedCapAt := strings.Index(query, "LIMIT $maxSeedNodes")
	if whereAt < 0 || seedCapAt < 0 || whereAt > seedCapAt {
		t.Errorf("the relationship check must precede the seed cap:\n%s", query)
	}

	if params["maxSeedNodes"] != graphSearchMaxSeedNodes {
		t.Errorf("maxSeedNodes = %v, want %d", params["maxSeedNodes"], graphSearchMaxSeedNodes)
	}
	if params["maxRows"] != graphSearchMaxRows {
		t.Errorf("maxRows = %v, want %d", params["maxRows"], graphSearchMaxRows)
	}
	if got, ok := params["nodes"].([]string); !ok || len(got) != 1 || got[0] != "安恒" {
		t.Errorf("nodes param = %#v, want []string{\"安恒\"}", params["nodes"])
	}

	// The label expression is interpolated rather than parameterised, so it has
	// to reach every match in the query.
	if !strings.Contains(query, "MATCH (n:ENTITY_kb)") || !strings.Contains(query, "MATCH (n)-[r]-(m:ENTITY_kb)") {
		t.Errorf("label expression not applied to both matches:\n%s", query)
	}
}

// -- fakes for the SearchNode decode path -----------------------------------
//
// SearchNode's decoding runs inside session.ExecuteRead, so the tests replay
// canned (n, r, m) records exactly as the server emits them for the undirected
// MATCH (n)-[r]-(m): one row per (seed, incident relationship), with n bound
// to the seed. Interfaces with unexported methods (Session, Result) are
// satisfied by embedding them; only the methods this path calls are
// implemented, so an unexpected call panics instead of passing silently.

type fakeDriver struct {
	neo4j.Driver
	rows []*neo4j.Record
}

func (d *fakeDriver) NewSession(ctx context.Context, config neo4j.SessionConfig) neo4j.Session {
	return &fakeSession{rows: d.rows}
}

type fakeSession struct {
	neo4j.Session
	rows []*neo4j.Record
}

func (s *fakeSession) ExecuteRead(ctx context.Context, work neo4j.ManagedTransactionWork, configurers ...func(*neo4j.TransactionConfig)) (any, error) {
	return work(&fakeTx{rows: s.rows})
}

func (s *fakeSession) Close(ctx context.Context) error { return nil }

type fakeTx struct {
	neo4j.ManagedTransaction
	rows []*neo4j.Record
}

func (t *fakeTx) Run(ctx context.Context, cypher string, params map[string]any) (neo4j.Result, error) {
	return &fakeResult{rows: t.rows}, nil
}

type fakeResult struct {
	neo4j.Result
	rows []*neo4j.Record
	next int
}

func (r *fakeResult) Next(ctx context.Context) bool {
	r.next++
	return r.next <= len(r.rows)
}

func (r *fakeResult) Record() *neo4j.Record {
	if r.next == 0 || r.next > len(r.rows) {
		return nil
	}
	return r.rows[r.next-1]
}

// -- issue #3945 fixture -----------------------------------------------------
//
// Two documents extract the same entity names, so the physical graph holds
// one node instance per (name, document) plus one relationship per document.
// The decoder must read direction from element ids, not from the row's column
// order, and must not collapse the two documents' instances into whichever
// name was seen first.

var (
	acmeDoc1     = fakeNode("n-acme-1", "Acme", []string{"c1"})
	shanghaiDoc1 = fakeNode("n-shanghai-1", "Shanghai", []string{"c1"})
	acmeDoc2     = fakeNode("n-acme-2", "Acme", []string{"c2"})
	shanghaiDoc2 = fakeNode("n-shanghai-2", "Shanghai", []string{"c2"})

	headquarteredIn = fakeRel("r-hq", acmeDoc1, shanghaiDoc1, "HEADQUARTERED_IN")
	hasBranchIn     = fakeRel("r-branch", acmeDoc2, shanghaiDoc2, "HAS_BRANCH_IN")
	// A genuinely inverse physical relationship: distinct element id, opposite
	// stored direction. Element-id deduplication must keep it.
	partnersWith = fakeRel("r-inv", shanghaiDoc1, acmeDoc1, "PARTNERS_WITH")
)

func fakeNode(elementID, name string, chunks []string) neo4j.Node {
	values := make([]any, len(chunks))
	for i, c := range chunks {
		values[i] = c
	}
	return neo4j.Node{
		ElementId: elementID,
		Props: map[string]any{
			"name":       name,
			"chunks":     values,
			"attributes": []any{},
		},
	}
}

func fakeRel(elementID string, start, end neo4j.Node, relType string) neo4j.Relationship {
	return neo4j.Relationship{
		ElementId:      elementID,
		StartElementId: start.ElementId,
		EndElementId:   end.ElementId,
		Type:           relType,
	}
}

func row(n neo4j.Node, r neo4j.Relationship, m neo4j.Node) *neo4j.Record {
	return &neo4j.Record{Keys: []string{"n", "r", "m"}, Values: []any{n, r, m}}
}

func searchNodeRows(t *testing.T, rows []*neo4j.Record) *types.GraphData {
	t.Helper()
	repo := NewNeo4jRepository(&fakeDriver{rows: rows})
	graph, err := repo.SearchNode(context.Background(),
		types.NameSpace{KnowledgeBase: "kb", Knowledge: "kg"}, []string{"Shanghai"})
	if err != nil {
		t.Fatalf("SearchNode failed: %v", err)
	}
	if graph == nil {
		t.Fatal("SearchNode returned nil graph")
	}
	return graph
}

func oneRelation(t *testing.T, graph *types.GraphData, relType string) *types.GraphRelation {
	t.Helper()
	var found []*types.GraphRelation
	for _, rel := range graph.Relation {
		if rel.Type == relType {
			found = append(found, rel)
		}
	}
	if len(found) != 1 {
		t.Fatalf("relation %s appears %d time(s), want 1; relations: %+v", relType, len(found), graph.Relation)
	}
	return found[0]
}

func oneNode(t *testing.T, graph *types.GraphData, name string) *types.GraphNode {
	t.Helper()
	var found []*types.GraphNode
	for _, node := range graph.Node {
		if node.Name == name {
			found = append(found, node)
		}
	}
	if len(found) != 1 {
		t.Fatalf("node %s appears %d time(s), want 1; nodes: %+v", name, len(found), graph.Node)
	}
	return found[0]
}

func assertList(t *testing.T, what string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s = %v, want %v", what, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s = %v, want %v", what, got, want)
		}
	}
}

// TestSearchNodeKeepsStoredDirectionWhenSeedIsTarget is issue #3945 (a): with
// seed=Shanghai every row's n is the Shanghai instance, i.e. the end node of
// HEADQUARTERED_IN and HAS_BRANCH_IN. Decoding by column order flipped those
// relations; decoding must follow the stored direction.
func TestSearchNodeKeepsStoredDirectionWhenSeedIsTarget(t *testing.T) {
	graph := searchNodeRows(t, []*neo4j.Record{
		row(shanghaiDoc1, headquarteredIn, acmeDoc1),
		row(shanghaiDoc1, partnersWith, acmeDoc1),
		row(shanghaiDoc2, hasBranchIn, acmeDoc2),
	})

	for _, tc := range []struct {
		relType      string
		node1, node2 string
		node1Chunks  []string
		node2Chunks  []string
	}{
		{"HEADQUARTERED_IN", "Acme", "Shanghai", []string{"c1"}, []string{"c1"}},
		{"PARTNERS_WITH", "Shanghai", "Acme", []string{"c1"}, []string{"c1"}},
		{"HAS_BRANCH_IN", "Acme", "Shanghai", []string{"c2"}, []string{"c2"}},
	} {
		rel := oneRelation(t, graph, tc.relType)
		if rel.Node1 != tc.node1 || rel.Node2 != tc.node2 {
			t.Errorf("%s decoded as %s->%s, want %s->%s (stored direction)",
				tc.relType, rel.Node1, rel.Node2, tc.node1, tc.node2)
		}
		// (d) endpoint-instance chunk evidence rides along on the relation.
		assertList(t, tc.relType+".Node1Chunks", rel.Node1Chunks, tc.node1Chunks)
		assertList(t, tc.relType+".Node2Chunks", rel.Node2Chunks, tc.node2Chunks)
	}

	// (c) both documents' instances merge into one entry per name, with both
	// documents' chunk evidence, in first-seen order.
	if len(graph.Node) != 2 {
		t.Fatalf("got %d node entries, want 2 (one per name): %+v", len(graph.Node), graph.Node)
	}
	assertList(t, "Acme.Chunks", oneNode(t, graph, "Acme").Chunks, []string{"c1", "c2"})
	assertList(t, "Shanghai.Chunks", oneNode(t, graph, "Shanghai").Chunks, []string{"c1", "c2"})
}

// TestSearchNodeDedupsPhysicalRelationsWhenBothEndpointsAreSeeds is issue
// #3945 (b): with both names as seeds, the undirected match reports each
// physical relationship once per seed — two opposite rows. Each physical
// relationship must come back exactly once, in its stored direction, while a
// genuinely inverse relationship (distinct element id) survives.
func TestSearchNodeDedupsPhysicalRelationsWhenBothEndpointsAreSeeds(t *testing.T) {
	graph := searchNodeRows(t, []*neo4j.Record{
		row(acmeDoc1, headquarteredIn, shanghaiDoc1),
		row(shanghaiDoc1, headquarteredIn, acmeDoc1), // same physical rel, opposite row
		row(acmeDoc1, partnersWith, shanghaiDoc1),
		row(shanghaiDoc1, partnersWith, acmeDoc1),
		row(acmeDoc2, hasBranchIn, shanghaiDoc2),
		row(shanghaiDoc2, hasBranchIn, acmeDoc2),
	})

	if len(graph.Relation) != 3 {
		t.Fatalf("got %d relations, want 3 (one per physical relationship): %+v",
			len(graph.Relation), graph.Relation)
	}

	hq := oneRelation(t, graph, "HEADQUARTERED_IN")
	if hq.Node1 != "Acme" || hq.Node2 != "Shanghai" {
		t.Errorf("HEADQUARTERED_IN decoded as %s->%s, want Acme->Shanghai", hq.Node1, hq.Node2)
	}
	assertList(t, "HEADQUARTERED_IN.Node1Chunks", hq.Node1Chunks, []string{"c1"})
	assertList(t, "HEADQUARTERED_IN.Node2Chunks", hq.Node2Chunks, []string{"c1"})

	inv := oneRelation(t, graph, "PARTNERS_WITH")
	if inv.Node1 != "Shanghai" || inv.Node2 != "Acme" {
		t.Errorf("PARTNERS_WITH decoded as %s->%s, want Shanghai->Acme (genuine inverse kept)",
			inv.Node1, inv.Node2)
	}
	assertList(t, "PARTNERS_WITH.Node1Chunks", inv.Node1Chunks, []string{"c1"})
	assertList(t, "PARTNERS_WITH.Node2Chunks", inv.Node2Chunks, []string{"c1"})

	br := oneRelation(t, graph, "HAS_BRANCH_IN")
	if br.Node1 != "Acme" || br.Node2 != "Shanghai" {
		t.Errorf("HAS_BRANCH_IN decoded as %s->%s, want Acme->Shanghai", br.Node1, br.Node2)
	}
	assertList(t, "HAS_BRANCH_IN.Node1Chunks", br.Node1Chunks, []string{"c2"})
	assertList(t, "HAS_BRANCH_IN.Node2Chunks", br.Node2Chunks, []string{"c2"})

	if len(graph.Node) != 2 {
		t.Fatalf("got %d node entries, want 2 (one per name): %+v", len(graph.Node), graph.Node)
	}
	assertList(t, "Acme.Chunks", oneNode(t, graph, "Acme").Chunks, []string{"c1", "c2"})
	assertList(t, "Shanghai.Chunks", oneNode(t, graph, "Shanghai").Chunks, []string{"c1", "c2"})
}
