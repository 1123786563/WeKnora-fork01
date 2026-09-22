package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/craft"
)

func shaHex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func inputScope() craft.Scope {
	return craft.Scope{TenantID: 7, UserID: "u-stage", SessionID: "s-stage"}
}

func inputWorkspace() craft.Workspace {
	return craft.Workspace{ID: "ws-1", Scope: inputScope(), SandboxID: "sbx-1"}
}

type stagedWrite struct {
	workspace craft.Workspace
	path      string
	content   []byte
}

type recordingWriter struct {
	writes []stagedWrite
	fail   error
}

func (w *recordingWriter) write(_ context.Context, ws craft.Workspace, path string, content []byte) error {
	if w.fail != nil {
		return w.fail
	}
	w.writes = append(w.writes, stagedWrite{workspace: ws, path: path, content: content})
	return nil
}

// authorizedLoader mimics the assembly-injected authorized resource read: it
// serves bytes for refs owned by the scope's tenant and answers
// craft.ErrForbidden for anything else, exactly like the permission-backed
// loader the handlers must install.
type authorizedLoader struct {
	tenantResources map[string][]byte
	reads           int
}

func (l *authorizedLoader) load(_ context.Context, scope craft.Scope, ref string) ([]byte, error) {
	l.reads++
	if content, ok := l.tenantResources[ref]; ok {
		return content, nil
	}
	return nil, fmt.Errorf("%w: ref %s is outside tenant %d", craft.ErrForbidden, ref, scope.TenantID)
}

func declaredInput(ref, name string, content []byte) craft.Input {
	return craft.Input{Ref: ref, Name: name, SHA256: shaHex(content), Bytes: int64(len(content)), CitationID: "cite-" + name}
}

func TestStageWritesContentAddressedPathsAndKeepsManifest(t *testing.T) {
	scope := inputScope()
	csv := []byte("month,sales\njan,10\n")
	pdf := []byte("%pdf-bytes")
	loader := &authorizedLoader{tenantResources: map[string][]byte{
		"res://tenant7/csv": csv,
		"res://tenant7/pdf": pdf,
	}}
	writer := &recordingWriter{}
	svc := NewCraftInputService(loader.load, writer.write)
	inputs := []craft.Input{
		declaredInput("res://tenant7/csv", "sales.csv", csv),
		declaredInput("res://tenant7/pdf", "Brief.pdf", pdf),
	}
	manifest, err := svc.Stage(context.Background(), scope, inputWorkspace(), inputs)
	if err != nil {
		t.Fatal(err)
	}
	if len(writer.writes) != 2 {
		t.Fatalf("writer called %d times, want 2", len(writer.writes))
	}
	for i, in := range inputs {
		wantPath := "inputs/" + in.SHA256 + "/" + in.Name
		if writer.writes[i].path != wantPath {
			t.Fatalf("write %d path = %q, want %q", i, writer.writes[i].path, wantPath)
		}
		if writer.writes[i].workspace.ID != "ws-1" || writer.writes[i].workspace.Scope != scope {
			t.Fatalf("write %d targeted workspace %#v", i, writer.writes[i].workspace)
		}
		if shaHex(writer.writes[i].content) != in.SHA256 {
			t.Fatalf("write %d content digest mismatch", i)
		}
	}
	if len(manifest) != 2 || manifest[0] != inputs[0] || manifest[1] != inputs[1] {
		t.Fatalf("manifest must preserve the source declarations, got %#v", manifest)
	}
}

func TestStageRejectsCrossTenantRefusalBeforeAnyWrite(t *testing.T) {
	scope := inputScope()
	own := []byte("kept")
	loader := &authorizedLoader{tenantResources: map[string][]byte{"res://tenant7/own": own}}
	writer := &recordingWriter{}
	svc := NewCraftInputService(loader.load, writer.write)
	_, err := svc.Stage(context.Background(), scope, inputWorkspace(), []craft.Input{
		declaredInput("res://tenant7/own", "own.txt", own),
		declaredInput("res://tenant9/secret", "secret.txt", []byte("x")),
	})
	if !errors.Is(err, craft.ErrForbidden) {
		t.Fatalf("error = %v, want craft.ErrForbidden", err)
	}
	if len(writer.writes) != 0 {
		t.Fatalf("writer called %d times after a cross-tenant refusal, want 0", len(writer.writes))
	}
}

func TestStageRejectsTamperedDeclaration(t *testing.T) {
	scope := inputScope()
	content := []byte("actual bytes")
	loader := &authorizedLoader{tenantResources: map[string][]byte{"res://tenant7/f": content}}
	writer := &recordingWriter{}
	svc := NewCraftInputService(loader.load, writer.write)

	tamperedDigest := declaredInput("res://tenant7/f", "f.txt", content)
	tamperedDigest.SHA256 = shaHex([]byte("other bytes"))
	_, err := svc.Stage(context.Background(), scope, inputWorkspace(), []craft.Input{tamperedDigest})
	if !errors.Is(err, craft.ErrInvalidInput) {
		t.Fatalf("digest mismatch error = %v, want craft.ErrInvalidInput", err)
	}

	tamperedSize := declaredInput("res://tenant7/f", "f.txt", content)
	tamperedSize.Bytes = int64(len(content) + 1)
	_, err = svc.Stage(context.Background(), scope, inputWorkspace(), []craft.Input{tamperedSize})
	if !errors.Is(err, craft.ErrInvalidInput) {
		t.Fatalf("size mismatch error = %v, want craft.ErrInvalidInput", err)
	}

	malformedDigest := declaredInput("res://tenant7/f", "f.txt", content)
	malformedDigest.SHA256 = "not-a-digest"
	_, err = svc.Stage(context.Background(), scope, inputWorkspace(), []craft.Input{malformedDigest})
	if !errors.Is(err, craft.ErrInvalidInput) {
		t.Fatalf("malformed digest error = %v, want craft.ErrInvalidInput", err)
	}

	_, err = svc.Stage(context.Background(), scope, inputWorkspace(), []craft.Input{
		declaredInput("res://tenant7/f", "../escape.txt", content)})
	if !errors.Is(err, craft.ErrInvalidInput) {
		t.Fatalf("escaping name error = %v, want craft.ErrInvalidInput", err)
	}
	if loader.reads != 2 {
		t.Fatalf("loader read %d times; only the two tampered-content cases may read", loader.reads)
	}
	if len(writer.writes) != 0 {
		t.Fatalf("writer called %d times after tampered declarations, want 0", len(writer.writes))
	}
}

func TestStageRejectsOverBudgetBeforeReading(t *testing.T) {
	scope := inputScope()
	content := []byte("x")
	loader := &authorizedLoader{tenantResources: map[string][]byte{"res://tenant7/f": content}}
	writer := &recordingWriter{}
	svc := NewCraftInputService(loader.load, writer.write)

	oversize := declaredInput("res://tenant7/f", "big.bin", content)
	oversize.Bytes = craft.MaxInputBytes + 1
	_, err := svc.Stage(context.Background(), scope, inputWorkspace(), []craft.Input{oversize})
	if !errors.Is(err, craft.ErrInvalidInput) {
		t.Fatalf("per-file over-budget error = %v, want craft.ErrInvalidInput", err)
	}

	tooMany := make([]craft.Input, craft.MaxInputsPerRound+1)
	for i := range tooMany {
		tooMany[i] = declaredInput("res://tenant7/f", fmt.Sprintf("f%d.txt", i), content)
	}
	_, err = svc.Stage(context.Background(), scope, inputWorkspace(), tooMany)
	if !errors.Is(err, craft.ErrInvalidInput) {
		t.Fatalf("count over-budget error = %v, want craft.ErrInvalidInput", err)
	}

	partBytes := int64(18 << 20)
	overTotal := make([]craft.Input, 0, 6)
	for i := 0; i < 6; i++ {
		overTotal = append(overTotal, craft.Input{
			Ref: "res://tenant7/f", Name: fmt.Sprintf("part%d.bin", i),
			SHA256: shaHex(content), Bytes: partBytes,
		})
	}
	_, err = svc.Stage(context.Background(), scope, inputWorkspace(), overTotal)
	if !errors.Is(err, craft.ErrInvalidInput) {
		t.Fatalf("total over-budget error = %v, want craft.ErrInvalidInput", err)
	}

	if loader.reads != 0 {
		t.Fatalf("loader read %d times for over-budget requests, want 0", loader.reads)
	}
	if len(writer.writes) != 0 {
		t.Fatalf("writer called %d times, want 0", len(writer.writes))
	}
}

func TestStageRechecksCapsOnActuallyReadBytes(t *testing.T) {
	scope := inputScope()
	// The declaration is small and well-formed; the loader misbehaves and
	// returns far more than declared, so only the post-read cap can catch it.
	huge := bytes.Repeat([]byte{0}, craft.MaxInputBytes+1)
	declared := craft.Input{
		Ref: "res://tenant7/f", Name: "f.bin",
		SHA256: shaHex([]byte("x")), Bytes: 1,
	}
	loader := &authorizedLoader{tenantResources: map[string][]byte{"res://tenant7/f": huge}}
	writer := &recordingWriter{}
	svc := NewCraftInputService(loader.load, writer.write)
	_, err := svc.Stage(context.Background(), scope, inputWorkspace(), []craft.Input{declared})
	if !errors.Is(err, craft.ErrInvalidInput) {
		t.Fatalf("post-read over-budget error = %v, want craft.ErrInvalidInput", err)
	}
	if len(writer.writes) != 0 {
		t.Fatalf("writer called %d times after an over-budget read, want 0", len(writer.writes))
	}
}

func TestStageRejectsForeignWorkspace(t *testing.T) {
	loader := &authorizedLoader{tenantResources: map[string][]byte{"res://tenant7/f": []byte("x")}}
	writer := &recordingWriter{}
	svc := NewCraftInputService(loader.load, writer.write)
	foreign := inputWorkspace()
	foreign.Scope.UserID = "someone-else"
	_, err := svc.Stage(context.Background(), inputScope(), foreign,
		[]craft.Input{declaredInput("res://tenant7/f", "f.txt", []byte("x"))})
	if !errors.Is(err, craft.ErrForbidden) {
		t.Fatalf("error = %v, want craft.ErrForbidden", err)
	}
	if loader.reads != 0 || len(writer.writes) != 0 {
		t.Fatalf("foreign workspace must fail before any read or write")
	}
}
