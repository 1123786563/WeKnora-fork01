package connectorcontrol

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testKeyMaterial returns 32 raw key bytes encoded as 64 hex chars (the
// documented provisioning format) and the bytes themselves.
func testKeyMaterial() (string, []byte) {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i * 7)
	}
	return hex.EncodeToString(raw), raw
}

// writeTestKey provisions a perm-enforced key file the way a mounted secret
// arrives: 0400, single line, trimmed on read. Each call uses a fresh name
// (a 0400 file cannot be rewritten in place).
var testKeySeq int

func writeTestKey(t *testing.T, dir string, material string) string {
	t.Helper()
	testKeySeq++
	path := filepath.Join(dir, fmt.Sprintf("connector-secret-key-%d", testKeySeq))
	if err := os.WriteFile(path, append([]byte(material), '\n'), 0o400); err != nil {
		t.Fatal(err)
	}
	return path
}

// tDir0700 provisions a temp dir with the permissions the sink enforces
// (t.TempDir may arrive 0755 depending on platform umask).
func tDir0700(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func newTestSink(t *testing.T, keyPath string) *EncryptedFileSecretSink {
	t.Helper()
	sink, err := NewEncryptedFileSecretSink(tDir0700(t), FileSecretKeySource(keyPath))
	if err != nil {
		t.Fatal(err)
	}
	return sink
}

// TestEncryptedFileSecretSinkRoundTripSealsAtRest pins the core contract:
// what PutSecret wrote is readable back through GetSecret, but the file on
// disk carries no plaintext — the at-rest guarantee is byte-level, not
// assumption-level.
func TestEncryptedFileSecretSinkRoundTripSealsAtRest(t *testing.T) {
	dir := t.TempDir()
	material, _ := testKeyMaterial()
	keyPath := writeTestKey(t, dir, material)
	sink := newTestSink(t, keyPath)
	ctx := context.Background()

	ref := OCTestSinkRef
	if err := sink.PutSecret(ctx, ref, "oct_restricted_material"); err != nil {
		t.Fatal(err)
	}
	got, err := sink.GetSecret(ctx, ref)
	if err != nil || got != "oct_restricted_material" {
		t.Fatalf("got=%q err=%v", got, err)
	}
	data, err := os.ReadFile(filepath.Join(sink.dir, secretFileName(ref)))
	if err != nil {
		t.Fatal(err)
	}
	record := string(data)
	if !strings.HasPrefix(record, encryptedSecretPrefix) {
		t.Fatalf("record on disk is not sealed: %q", record)
	}
	if strings.Contains(record, "oct_restricted_material") {
		t.Fatal("plaintext leaked into the sealed record")
	}
	if st, _ := os.Stat(filepath.Join(sink.dir, secretFileName(ref))); st.Mode().Perm() != 0o600 {
		t.Fatalf("sealed record mode = %04o, want 0600", st.Mode().Perm())
	}
}

// OCTestSinkRef is a stable reference shape for sink tests (mirrors the
// production runtime-token refs: oc/runtime-token/<tenant>/<conn>/<version>).
const OCTestSinkRef = "oc/runtime-token/7/conn-1/3"

// TestEncryptedFileSecretSinkWorkerAPIShardInterop pins the deployment
// parity: material the CONTROL WORKER seals is read back by a second sink
// instance (the API process read side) provisioned with the same key.
func TestEncryptedFileSecretSinkWorkerAPIShardInterop(t *testing.T) {
	dir := t.TempDir()
	material, _ := testKeyMaterial()
	keyPath := writeTestKey(t, dir, material)
	worker, err := NewEncryptedFileSecretSink(tDir0700(t), FileSecretKeySource(keyPath))
	if err != nil {
		t.Fatal(err)
	}
	api, err := NewEncryptedFileSecretSink(worker.dir, FileSecretKeySource(keyPath))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := worker.PutSecret(ctx, OCTestSinkRef, "oct_shared_material"); err != nil {
		t.Fatal(err)
	}
	got, err := api.GetSecret(ctx, OCTestSinkRef)
	if err != nil || got != "oct_shared_material" {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

// TestEncryptedFileSecretSinkWrongKeyFailsClosed: a rotated-away key must
// never open old records (GCM authentication), and the error never echoes
// key or material.
func TestEncryptedFileSecretSinkWrongKeyFailsClosed(t *testing.T) {
	dir := t.TempDir()
	material, _ := testKeyMaterial()
	keyPath := writeTestKey(t, dir, material)
	sink := newTestSink(t, keyPath)
	ctx := context.Background()
	if err := sink.PutSecret(ctx, OCTestSinkRef, "oct_material"); err != nil {
		t.Fatal(err)
	}
	other := make([]byte, 32)
	for i := range other {
		other[i] = byte(255 - i)
	}
	otherPath := writeTestKey(t, dir, hex.EncodeToString(other))
	otherSink := newTestSink(t, otherPath)
	otherSink.dir = sink.dir
	if _, err := otherSink.GetSecret(ctx, OCTestSinkRef); err == nil {
		t.Fatal("record opened under the wrong key")
	}
}

// TestEncryptedFileSecretSinkRejectsLegacyPlaintext: a phase-one record on
// disk is refused, never silently served.
func TestEncryptedFileSecretSinkRejectsLegacyPlaintext(t *testing.T) {
	dir := t.TempDir()
	material, _ := testKeyMaterial()
	keyPath := writeTestKey(t, dir, material)
	sink := newTestSink(t, keyPath)
	legacy, err := NewFileSecretSink(sink.dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := legacy.PutSecret(ctx, OCTestSinkRef, "oct_plaintext_legacy"); err != nil {
		t.Fatal(err)
	}
	if _, err := sink.GetSecret(ctx, OCTestSinkRef); err == nil || !strings.Contains(err.Error(), "not encrypted") {
		t.Fatalf("legacy plaintext not refused: %v", err)
	}
}

// TestEncryptedFileSecretSinkMissingRecordFailsClosed: no reference, no
// empty-string success.
func TestEncryptedFileSecretSinkMissingRecordFailsClosed(t *testing.T) {
	dir := t.TempDir()
	material, _ := testKeyMaterial()
	sink := newTestSink(t, writeTestKey(t, dir, material))
	if _, err := sink.GetSecret(context.Background(), "oc/runtime-token/1/absent/1"); err == nil {
		t.Fatal("missing record must fail closed")
	}
}

// TestEncryptedFileSecretSinkKeySourcePermEnforced: an open-permission key
// mount refuses to yield the key (mirrors FileAdminSecretSource).
func TestEncryptedFileSecretSinkKeySourcePermEnforced(t *testing.T) {
	dir := t.TempDir()
	material, _ := testKeyMaterial()
	path := filepath.Join(dir, "open-key")
	if err := os.WriteFile(path, []byte(material), 0o644); err != nil {
		t.Fatal(err)
	}
	source := FileSecretKeySource(path)
	if _, err := source(context.Background()); err == nil || !strings.Contains(err.Error(), "too open") {
		t.Fatalf("open-permission key not refused: %v", err)
	}
}

// TestEncryptedFileSecretSinkKeyFormatsAccepted: hex, base64 (std and URL,
// padded and raw) and raw 32-byte material all provision the same key;
// anything else is refused.
func TestEncryptedFileSecretSinkKeyFormatsAccepted(t *testing.T) {
	_, raw := testKeyMaterial()
	materials := []string{
		hex.EncodeToString(raw),
		base64.StdEncoding.EncodeToString(raw),
		base64.RawStdEncoding.EncodeToString(raw),
		base64.URLEncoding.EncodeToString(raw),
		base64.RawURLEncoding.EncodeToString(raw),
		string(raw),
	}
	for _, m := range materials {
		key, err := parseSecretKey([]byte(m))
		if err != nil || hex.EncodeToString(key) != hex.EncodeToString(raw) {
			t.Fatalf("material %q: key=%x err=%v", m, key, err)
		}
	}
	for _, bad := range []string{"", "tooshort", strings.Repeat("z", 64), base64.StdEncoding.EncodeToString([]byte("31 bytes of key material here!!"))} {
		if _, err := parseSecretKey([]byte(bad)); err == nil {
			t.Fatalf("bad material %q accepted", bad)
		}
	}
}

// TestEncryptedFileSecretSinkDirPermsEnforced: the constructor refuses a
// group/world-accessible secrets directory — the real T05-Q-01 fix.
func TestEncryptedFileSecretSinkDirPermsEnforced(t *testing.T) {
	dir := t.TempDir()
	material, _ := testKeyMaterial()
	keyPath := writeTestKey(t, dir, material)
	open := t.TempDir()
	if err := os.Chmod(open, 0o750); err != nil {
		t.Fatal(err)
	}
	if _, err := NewEncryptedFileSecretSink(open, FileSecretKeySource(keyPath)); err == nil || !strings.Contains(err.Error(), "too open") {
		t.Fatalf("open secrets dir not refused: %v", err)
	}
}

// TestEncryptedFileSecretSinkRequiresKeySource: no key, no sink — static
// encryption is not optional for this constructor.
func TestEncryptedFileSecretSinkRequiresKeySource(t *testing.T) {
	if _, err := NewEncryptedFileSecretSink(t.TempDir(), nil); err == nil {
		t.Fatal("nil key source accepted")
	}
	if _, err := NewEncryptedFileSecretSink("", FileSecretKeySource("x")); err == nil {
		t.Fatal("empty dir accepted")
	}
}

// TestEncryptedFileSecretSinkTamperedRecordFails: flipping a ciphertext byte
// breaks the GCM tag and the record refuses to open.
func TestEncryptedFileSecretSinkTamperedRecordFails(t *testing.T) {
	dir := t.TempDir()
	material, _ := testKeyMaterial()
	keyPath := writeTestKey(t, dir, material)
	sink := newTestSink(t, keyPath)
	ctx := context.Background()
	if err := sink.PutSecret(ctx, OCTestSinkRef, "oct_material"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(sink.dir, secretFileName(OCTestSinkRef))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tampered := []byte(data)
	tampered[len(tampered)-3] ^= 0x01
	if err := os.WriteFile(path, tampered, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := sink.GetSecret(ctx, OCTestSinkRef); err == nil {
		t.Fatal("tampered record opened")
	}
}
