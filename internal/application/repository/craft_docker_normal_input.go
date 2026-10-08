package repository

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const MaxCraftDockerNormalStdinBytes = 1 << 20
const MaxCraftDockerNormalRequestBytes = 2 << 20

// AES-GCM stores a 12-byte nonce and 16-byte tag before raw URL-base64 encoding.
const MaxCraftDockerNormalCiphertextBytes = len(utils.EncPrefix) + 2796240

var (
	ErrCraftDockerNormalInputConflict       = fmt.Errorf("Docker normal input conflicts with durable identity: %w", craft.ErrConflict)
	ErrCraftDockerNormalInputNotFound       = errors.New("Docker normal input not found")
	ErrCraftDockerNormalInputKeyUnavailable = errors.New("Docker normal input encryption requires a valid SYSTEM_AES_KEY")
	ErrCraftDockerNormalInputCorrupt        = errors.New("Docker normal input failed integrity or decryption check")
	ErrCraftDockerNormalInputTooLarge       = errors.New("Docker normal input exceeds the durable request size limit")
	ErrCraftDockerNormalInputInvalid        = fmt.Errorf("Docker normal input request shape is invalid: %w", craft.ErrInvalidInput)
	ErrCraftDockerNormalInputUnavailable    = errors.New("Docker normal input store is unavailable")
)

type CraftDockerNormalInputRequest struct {
	TenantID      uint64            `json:"tenant_id"`
	TaskID        string            `json:"task_id"`
	RunID         string            `json:"run_id"`
	ActivityKey   string            `json:"activity_key"`
	Command       []string          `json:"command"`
	Environment   map[string]string `json:"environment"`
	User          string            `json:"user"`
	WorkingDir    string            `json:"working_dir"`
	TimeoutMillis int64             `json:"timeout_ms"`
	StdinEnabled  bool              `json:"stdin_enabled"`
	Stdin         []byte            `json:"stdin"`
	OutputLimit   int64             `json:"output_limit"`
	OutputPolicy  string            `json:"output_policy"`

	// ResolvedTargetPath / TargetSHA256 are OPTIONAL additive fields for
	// server-resolvable faces (the fixed-shape web build entry): such a face
	// walks the HOST filesystem, fills the symlink-resolved absolute target
	// and its digest, and the T03 module policy's identity layer fires
	// exactly as it does for adapter-supplied evidence. Container-side
	// callers leave both empty — the canonical JSON form and the durable
	// encrypted identity are unchanged.
	ResolvedTargetPath string `json:"resolved_target_path,omitempty"`
	TargetSHA256       string `json:"target_sha256,omitempty"`
}

type CraftDockerStagedNormalInput struct {
	Request        CraftDockerNormalInputRequest
	RequestSHA256  string
	StdinEnabled   bool
	StdinByteCount int64
	StdinSHA256    string
	Receipt        *CraftDockerNormalReceipt
}

type craftDockerNormalInputRow struct {
	TenantID              uint64  `gorm:"column:tenant_id;primaryKey"`
	TaskID                string  `gorm:"column:task_id"`
	RunID                 string  `gorm:"column:run_id;primaryKey"`
	ActivityKey           string  `gorm:"column:activity_key;primaryKey"`
	RequestCiphertext     string  `gorm:"column:request_ciphertext"`
	RequestSHA256         string  `gorm:"column:request_sha256"`
	StdinEnabled          bool    `gorm:"column:stdin_enabled"`
	StdinByteCount        int64   `gorm:"column:stdin_byte_count"`
	StdinSHA256           string  `gorm:"column:stdin_sha256"`
	TimeoutMillis         int64   `gorm:"column:timeout_ms"`
	OutputLimit           int64   `gorm:"column:output_limit"`
	OutputPolicy          string  `gorm:"column:output_policy"`
	Provider              *string `gorm:"column:provider"`
	ContainerID           *string `gorm:"column:container_id"`
	ExecID                *string `gorm:"column:exec_id"`
	ReceiptStdinEnabled   *bool   `gorm:"column:receipt_stdin_enabled"`
	ReceiptStdinByteCount *int64  `gorm:"column:receipt_stdin_byte_count"`
	ReceiptStdinSHA256    *string `gorm:"column:receipt_stdin_sha256"`
	ReceiptTimeoutMillis  *int64  `gorm:"column:receipt_timeout_ms"`
}

func (craftDockerNormalInputRow) TableName() string { return "craft_docker_normal_inputs" }

type CraftDockerNormalInputRepository struct{ db *gorm.DB }

func NewCraftDockerNormalInputRepository(db *gorm.DB) *CraftDockerNormalInputRepository {
	return &CraftDockerNormalInputRepository{db: db}
}

// Stage creates the encrypted request before any Docker create call. Replaying
// an exact canonical request is idempotent; a changed request cannot inherit
// the operation's admission/hold.
// dockerNormalInputDBError wraps infrastructure failures of the normal-input
// chain with the INPUT-domain unavailable sentinel (not the output-domain
// one) so diagnostics are not misread as output-store outages.
func dockerNormalInputDBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrCraftDockerNormalInputConflict) || errors.Is(err, ErrCraftDockerNormalInputNotFound) ||
		errors.Is(err, ErrCraftDockerNormalInputTooLarge) || errors.Is(err, ErrCraftDockerNormalInputInvalid) ||
		errors.Is(err, ErrCraftDockerNormalInputCorrupt) || errors.Is(err, ErrCraftDockerNormalInputKeyUnavailable) {
		return err
	}
	return fmt.Errorf("%w: %v", ErrCraftDockerNormalInputUnavailable, err)
}

func (r *CraftDockerNormalInputRepository) Stage(ctx context.Context, request CraftDockerNormalInputRequest) (CraftDockerStagedNormalInput, error) {
	if r == nil || r.db == nil {
		return CraftDockerStagedNormalInput{}, ErrCraftDockerNormalInputConflict
	}
	if !validCraftDockerNormalInput(request) {
		// A malformed request (NUL bytes, env keys containing '=', out-of-range
		// timeout) conflicts with nothing: no durable identity exists yet.
		// The input-class error keeps callers from hunting a phantom conflict.
		return CraftDockerStagedNormalInput{}, fmt.Errorf("%w: %s", ErrCraftDockerNormalInputInvalid, "normal input request shape is invalid")
	}
	canonicalSize := craftDockerNormalCanonicalJSONSize(request)
	if canonicalSize > MaxCraftDockerNormalRequestBytes {
		return CraftDockerStagedNormalInput{}, ErrCraftDockerNormalInputTooLarge
	}
	key := CraftChargeStartKey{TenantID: request.TenantID, RunID: request.RunID, ActivityKey: request.ActivityKey}
	plain, err := json.Marshal(request)
	if err != nil {
		return CraftDockerStagedNormalInput{}, err
	}
	if int64(len(plain)) != canonicalSize {
		// The size calculator's internal invariant broke — that is a
		// calculator regression, not user input being too large.
		return CraftDockerStagedNormalInput{}, fmt.Errorf("%w: canonical size calculator disagrees with json.Marshal", ErrCraftDockerNormalInputCorrupt)
	}
	if int64(len(plain)) > MaxCraftDockerNormalRequestBytes {
		return CraftDockerStagedNormalInput{}, ErrCraftDockerNormalInputTooLarge
	}
	keyBytes := utils.GetAESKey()
	if len(keyBytes) != 32 {
		return CraftDockerStagedNormalInput{}, ErrCraftDockerNormalInputKeyUnavailable
	}
	ciphertext, err := utils.EncryptAESGCM(string(plain), keyBytes)
	if err != nil {
		return CraftDockerStagedNormalInput{}, err
	}
	if ciphertext == string(plain) || !strings.HasPrefix(ciphertext, utils.EncPrefix) {
		return CraftDockerStagedNormalInput{}, ErrCraftDockerNormalInputKeyUnavailable
	}
	if len(ciphertext) > MaxCraftDockerNormalCiphertextBytes {
		return CraftDockerStagedNormalInput{}, ErrCraftDockerNormalInputTooLarge
	}
	requestDigest := normalSHA256(plain)
	row := craftDockerNormalInputRow{TenantID: request.TenantID, TaskID: request.TaskID, RunID: request.RunID, ActivityKey: request.ActivityKey,
		RequestCiphertext: ciphertext, RequestSHA256: requestDigest, StdinEnabled: request.StdinEnabled, StdinByteCount: int64(len(request.Stdin)),
		StdinSHA256: normalSHA256(request.Stdin), TimeoutMillis: request.TimeoutMillis, OutputLimit: request.OutputLimit, OutputPolicy: request.OutputPolicy}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockDockerNormalInputRun(tx, ctx, key); err != nil {
			return err
		}
		var run struct {
			SessionID string
			Revision  int64
		}
		if err := tx.Table("agent_runs").Select("session_id, revision").Where("tenant_id = ? AND run_id = ?", key.TenantID, key.RunID).Take(&run).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCraftDockerNormalInputConflict
			}
			return dockerNormalInputDBError(err)
		}
		if run.SessionID != request.TaskID {
			return ErrCraftDockerNormalInputConflict
		}
		var prior craftDockerNormalInputRow
		lookup := tx.Where("tenant_id = ? AND run_id = ? AND activity_key = ?", key.TenantID, key.RunID, key.ActivityKey).Take(&prior)
		if lookup.Error == nil {
			staged, err := decryptCraftDockerNormalInput(prior)
			if err != nil {
				return err
			}
			if staged.RequestSHA256 != requestDigest || !reflect.DeepEqual(staged.Request, request) {
				return ErrCraftDockerNormalInputConflict
			}
			return nil
		}
		if !errors.Is(lookup.Error, gorm.ErrRecordNotFound) {
			return dockerNormalInputDBError(lookup.Error)
		}
		var journal struct {
			State         string
			Protocol      *string
			Provider      *string
			SendClaimedAt *time.Time
			RunRevision   int64
		}
		if err := tx.Table("craft_charge_start_journal").Select("state, protocol, provider, send_claimed_at, run_revision").Where(
			"tenant_id = ? AND run_id = ? AND activity_key = ?", key.TenantID, key.RunID, key.ActivityKey).Take(&journal).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCraftDockerNormalInputConflict
			}
			return dockerNormalInputDBError(err)
		}
		if journal.State != "intent" || journal.Protocol == nil || *journal.Protocol != "docker_coordinator" || journal.Provider != nil || journal.SendClaimedAt != nil || journal.RunRevision != run.Revision {
			return ErrCraftDockerNormalInputConflict
		}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
		if created.Error != nil {
			return dockerNormalInputDBError(created.Error)
		}
		return nil
	})
	if err != nil {
		return CraftDockerStagedNormalInput{}, err
	}
	stored, err := r.load(ctx, key)
	if err != nil {
		return CraftDockerStagedNormalInput{}, err
	}
	staged, err := decryptCraftDockerNormalInput(stored)
	if err != nil {
		return CraftDockerStagedNormalInput{}, err
	}
	if staged.RequestSHA256 != requestDigest || !reflect.DeepEqual(staged.Request, request) {
		return CraftDockerStagedNormalInput{}, ErrCraftDockerNormalInputConflict
	}
	return staged, nil
}

// Read reconstructs the exact encrypted create request under its tenant/Run
// scope and verifies both its canonical request digest and stdin identity.
func (r *CraftDockerNormalInputRepository) Read(ctx context.Context, key CraftChargeStartKey) (CraftDockerStagedNormalInput, error) {
	if r == nil || r.db == nil || key.TenantID == 0 || strings.TrimSpace(key.RunID) == "" || strings.TrimSpace(key.ActivityKey) == "" {
		return CraftDockerStagedNormalInput{}, ErrCraftDockerNormalInputConflict
	}
	row, err := r.load(ctx, key)
	if err != nil {
		return CraftDockerStagedNormalInput{}, err
	}
	var run struct{ SessionID string }
	if err := r.db.WithContext(ctx).Table("agent_runs").Select("session_id").Where("tenant_id = ? AND run_id = ?", key.TenantID, key.RunID).Take(&run).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return CraftDockerStagedNormalInput{}, ErrCraftDockerNormalInputConflict
		}
		return CraftDockerStagedNormalInput{}, dockerNormalInputDBError(err)
	}
	if run.SessionID != row.TaskID {
		return CraftDockerStagedNormalInput{}, ErrCraftDockerNormalInputConflict
	}
	return decryptCraftDockerNormalInput(row)
}

func (r *CraftDockerNormalInputRepository) load(ctx context.Context, key CraftChargeStartKey) (craftDockerNormalInputRow, error) {
	var row craftDockerNormalInputRow
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND run_id = ? AND activity_key = ?", key.TenantID, key.RunID, key.ActivityKey).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return craftDockerNormalInputRow{}, ErrCraftDockerNormalInputNotFound
	}
	if err != nil {
		return craftDockerNormalInputRow{}, dockerNormalInputDBError(err)
	}
	return row, nil
}

func decryptCraftDockerNormalInput(row craftDockerNormalInputRow) (CraftDockerStagedNormalInput, error) {
	if len(row.RequestCiphertext) > MaxCraftDockerNormalCiphertextBytes || !strings.HasPrefix(row.RequestCiphertext, utils.EncPrefix) {
		return CraftDockerStagedNormalInput{}, ErrCraftDockerNormalInputCorrupt
	}
	key := utils.GetAESKey()
	if len(key) != 32 {
		return CraftDockerStagedNormalInput{}, ErrCraftDockerNormalInputKeyUnavailable
	}
	plain, err := utils.DecryptAESGCM(row.RequestCiphertext, key)
	if err != nil {
		return CraftDockerStagedNormalInput{}, ErrCraftDockerNormalInputCorrupt
	}
	if int64(len(plain)) > MaxCraftDockerNormalRequestBytes {
		return CraftDockerStagedNormalInput{}, ErrCraftDockerNormalInputCorrupt
	}
	if normalSHA256([]byte(plain)) != row.RequestSHA256 {
		return CraftDockerStagedNormalInput{}, ErrCraftDockerNormalInputCorrupt
	}
	var request CraftDockerNormalInputRequest
	if err := json.Unmarshal([]byte(plain), &request); err != nil || !validCraftDockerNormalInput(request) || craftDockerNormalCanonicalJSONSize(request) != int64(len(plain)) || request.TenantID != row.TenantID || request.TaskID != row.TaskID || request.RunID != row.RunID || request.ActivityKey != row.ActivityKey || request.StdinEnabled != row.StdinEnabled || int64(len(request.Stdin)) != row.StdinByteCount || normalSHA256(request.Stdin) != row.StdinSHA256 || request.TimeoutMillis != row.TimeoutMillis || request.OutputLimit != row.OutputLimit || request.OutputPolicy != row.OutputPolicy {
		return CraftDockerStagedNormalInput{}, ErrCraftDockerNormalInputCorrupt
	}
	out := CraftDockerStagedNormalInput{Request: request, RequestSHA256: row.RequestSHA256, StdinEnabled: row.StdinEnabled,
		StdinByteCount: row.StdinByteCount, StdinSHA256: row.StdinSHA256}
	if row.Provider != nil {
		if row.ContainerID == nil || row.ExecID == nil || row.ReceiptStdinEnabled == nil || row.ReceiptStdinByteCount == nil || row.ReceiptStdinSHA256 == nil || row.ReceiptTimeoutMillis == nil {
			return CraftDockerStagedNormalInput{}, ErrCraftDockerNormalInputCorrupt
		}
		out.Receipt = &CraftDockerNormalReceipt{DockerExecReceipt: DockerExecReceipt{Provider: *row.Provider, ContainerID: *row.ContainerID, ExecID: *row.ExecID},
			StdinEnabled: *row.ReceiptStdinEnabled, StdinByteCount: *row.ReceiptStdinByteCount, StdinSHA256: *row.ReceiptStdinSHA256, TimeoutMillis: *row.ReceiptTimeoutMillis}
	}
	return out, nil
}

func validCraftDockerNormalInput(in CraftDockerNormalInputRequest) bool {
	if in.TenantID == 0 || strings.TrimSpace(in.TaskID) == "" || strings.TrimSpace(in.RunID) == "" || strings.TrimSpace(in.ActivityKey) == "" ||
		len(in.Command) > 256 || len(in.Environment) > 256 ||
		len(in.Command) == 0 || strings.TrimSpace(in.Command[0]) == "" || in.TimeoutMillis <= 0 || in.TimeoutMillis > 24*60*60*1000 ||
		in.OutputLimit <= 0 || in.OutputLimit > MaxCraftDockerOutputBytes || strings.TrimSpace(in.OutputPolicy) == "" || len(in.Stdin) > MaxCraftDockerNormalStdinBytes ||
		(!in.StdinEnabled && len(in.Stdin) > 0) {
		return false
	}
	for k, v := range in.Environment {
		if strings.TrimSpace(k) == "" || strings.ContainsAny(k, "=\x00") || strings.ContainsRune(v, '\x00') {
			return false
		}
	}
	// Additive evidence fields: a malformed TargetSHA256 silently misses the
	// manifest digest map (indistinguishable from "no evidence") while
	// suppressing the stdin channel's own identity derivation — it must be
	// exactly 64 lowercase hex when present, and both fields reject NUL.
	if strings.ContainsRune(in.ResolvedTargetPath, '\x00') || strings.ContainsRune(in.TargetSHA256, '\x00') {
		return false
	}
	if in.TargetSHA256 != "" && !isCraftDockerNormalDigest(in.TargetSHA256) {
		return false
	}
	for _, arg := range in.Command {
		if strings.ContainsRune(arg, '\x00') {
			return false
		}
	}
	return !strings.ContainsRune(in.User, '\x00') && !strings.ContainsRune(in.WorkingDir, '\x00')
}

// craftDockerNormalCanonicalJSONSize computes encoding/json's exact compact
// byte length without materializing the serialized request. It lets Stage
// reject over-limit commands and environments before Marshal or encryption.
func craftDockerNormalCanonicalJSONSize(in CraftDockerNormalInputRequest) int64 {
	size := int64(len(`{"tenant_id":`)) + int64(len(strconv.FormatUint(in.TenantID, 10)))
	size += int64(len(`,"task_id":`)) + craftDockerJSONQuotedStringSize(in.TaskID)
	size += int64(len(`,"run_id":`)) + craftDockerJSONQuotedStringSize(in.RunID)
	size += int64(len(`,"activity_key":`)) + craftDockerJSONQuotedStringSize(in.ActivityKey)
	size += int64(len(`,"command":`))
	if in.Command == nil {
		size += 4
	} else {
		size += 2
		for i, item := range in.Command {
			if i > 0 {
				size++
			}
			size += craftDockerJSONQuotedStringSize(item)
		}
	}
	size += int64(len(`,"environment":`))
	if in.Environment == nil {
		size += 4
	} else {
		size += 2
		i := 0
		for k, v := range in.Environment {
			if i > 0 {
				size++
			}
			size += craftDockerJSONQuotedStringSize(k) + 1 + craftDockerJSONQuotedStringSize(v)
			i++
		}
	}
	size += int64(len(`,"user":`)) + craftDockerJSONQuotedStringSize(in.User)
	size += int64(len(`,"working_dir":`)) + craftDockerJSONQuotedStringSize(in.WorkingDir)
	size += int64(len(`,"timeout_ms":`)) + int64(len(strconv.FormatInt(in.TimeoutMillis, 10)))
	size += int64(len(`,"stdin_enabled":`))
	if in.StdinEnabled {
		size += 4
	} else {
		size += 5
	}
	size += int64(len(`,"stdin":`))
	if in.Stdin == nil {
		size += 4
	} else {
		size += 2 + int64(base64.StdEncoding.EncodedLen(len(in.Stdin)))
	}
	size += int64(len(`,"output_limit":`)) + int64(len(strconv.FormatInt(in.OutputLimit, 10)))
	size += int64(len(`,"output_policy":`)) + craftDockerJSONQuotedStringSize(in.OutputPolicy)
	// Optional additive evidence fields (omitempty): each contributes its
	// `,"name":` prefix plus the JSON-quoted value ONLY when non-empty.
	if in.ResolvedTargetPath != "" {
		size += int64(len(`,"resolved_target_path":`)) + craftDockerJSONQuotedStringSize(in.ResolvedTargetPath)
	}
	if in.TargetSHA256 != "" {
		size += int64(len(`,"target_sha256":`)) + craftDockerJSONQuotedStringSize(in.TargetSHA256)
	}
	// closing brace
	size += 1
	return size
}

// isCraftDockerNormalDigest reports whether s is exactly 64 lowercase hex
// characters (a sha256 digest in the admitted-manifest identity space).
func isCraftDockerNormalDigest(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

func craftDockerJSONQuotedStringSize(value string) int64 {
	var size int64 = 2 // surrounding quotes
	for i := 0; i < len(value); {
		r, width := utf8.DecodeRuneInString(value[i:])
		switch {
		case width == 1 && r == utf8.RuneError:
			size += 6 // encoding/json emits the escaped replacement sequence \\ufffd
		case r == '"' || r == '\\' || r == '\n' || r == '\r' || r == '\t':
			// encoding/json emits \b and \f as the 6-byte \u0008/\u000c
			// escapes; only \" \\ \n \r \t use the 2-byte short form.
			size += 2
		case r < 0x20 || r == '<' || r == '>' || r == '&' || r == '\u2028' || r == '\u2029':
			size += 6
		default:
			size += int64(width)
		}
		i += width
	}
	return size
}

func normalSHA256(bytes []byte) string {
	sum := sha256.Sum256(bytes)
	return hex.EncodeToString(sum[:])
}

func lockDockerNormalInputRun(db *gorm.DB, ctx context.Context, key CraftChargeStartKey) error {
	locked := db.WithContext(ctx).Table("agent_runs").Where("tenant_id = ? AND run_id = ?", key.TenantID, key.RunID).UpdateColumn("revision", gorm.Expr("revision"))
	if locked.Error != nil {
		return dockerNormalInputDBError(locked.Error)
	}
	if locked.RowsAffected != 1 {
		return ErrCraftDockerNormalInputConflict
	}
	var run struct{ Status string }
	if err := db.WithContext(ctx).Table("agent_runs").Select("status").Where("tenant_id = ? AND run_id = ?", key.TenantID, key.RunID).Take(&run).Error; err != nil {
		return dockerNormalInputDBError(err)
	}
	if run.Status != "queued" && run.Status != "running" && run.Status != "recovering" {
		return ErrCraftDockerNormalInputConflict
	}
	return nil
}

// BindDockerNormalExecReceipt binds the provider receipt to the immutable
// staged request, then uses the pre-existing S2 Run-fenced receipt CAS.
func (r *CraftDockerSendClaimRepository) BindDockerNormalExecReceipt(ctx context.Context, key CraftChargeStartKey, revision int64, receipt CraftDockerNormalReceipt) error {
	if !validCraftDockerNormalReceipt(receipt) {
		return ErrCraftDockerNormalInputConflict
	}
	inputs := NewCraftDockerNormalInputRepository(r.db)
	staged, err := inputs.Read(ctx, key)
	if err != nil {
		return err
	}
	if !normalReceiptMatchesStage(receipt, staged) {
		return ErrCraftDockerNormalInputConflict
	}
	if err := r.bindDockerExecReceipt(ctx, key, revision, receipt.DockerExecReceipt, true); err != nil {
		return err
	}
	var affected int64
	result := r.db.WithContext(ctx).Table("craft_docker_normal_inputs").Where("tenant_id = ? AND run_id = ? AND activity_key = ? AND provider IS NULL AND receipt_stdin_enabled IS NULL", key.TenantID, key.RunID, key.ActivityKey).
		Updates(map[string]any{"provider": receipt.Provider, "container_id": receipt.ContainerID, "exec_id": receipt.ExecID,
			"receipt_stdin_enabled": receipt.StdinEnabled, "receipt_stdin_byte_count": receipt.StdinByteCount,
			"receipt_stdin_sha256": receipt.StdinSHA256, "receipt_timeout_ms": receipt.TimeoutMillis})
	if result.Error != nil {
		return dockerNormalInputDBError(result.Error)
	}
	affected = result.RowsAffected
	if affected == 1 {
		return nil
	}
	reloaded, err := inputs.Read(ctx, key)
	if err != nil {
		return err
	}
	if reloaded.Receipt != nil && *reloaded.Receipt == receipt {
		return nil
	}
	return ErrCraftDockerNormalInputConflict
}

// CraftDockerNormalReceipt includes the exact input/timeout identity used by
// ExecCreate. It is persisted before the durable send claim is granted.
type CraftDockerNormalReceipt struct {
	DockerExecReceipt
	StdinEnabled   bool
	StdinByteCount int64
	StdinSHA256    string
	TimeoutMillis  int64
}

func (r CraftDockerNormalReceipt) withStdinHash(hash string) CraftDockerNormalReceipt {
	r.StdinSHA256 = hash
	return r
}

func validCraftDockerNormalReceipt(r CraftDockerNormalReceipt) bool {
	if validateCraftDockerSendIdentity(CraftChargeStartKey{TenantID: 1, RunID: "run", ActivityKey: "activity"}, r.DockerExecReceipt) != nil || r.StdinByteCount < 0 || len(r.StdinSHA256) != 64 || r.TimeoutMillis <= 0 {
		return false
	}
	_, err := hex.DecodeString(r.StdinSHA256)
	return err == nil
}

func normalReceiptMatchesStage(receipt CraftDockerNormalReceipt, stage CraftDockerStagedNormalInput) bool {
	return receipt.StdinEnabled == stage.StdinEnabled && receipt.StdinByteCount == stage.StdinByteCount && receipt.StdinSHA256 == stage.StdinSHA256 && receipt.TimeoutMillis == stage.Request.TimeoutMillis
}

func (r *CraftDockerSendClaimRepository) ClaimDockerNormalExecSend(ctx context.Context, key CraftChargeStartKey, revision int64, receipt CraftDockerNormalReceipt) (bool, error) {
	if !validCraftDockerNormalReceipt(receipt) {
		return false, ErrCraftDockerNormalInputConflict
	}
	stage, err := NewCraftDockerNormalInputRepository(r.db).Read(ctx, key)
	if err != nil {
		return false, err
	}
	if stage.Receipt == nil || *stage.Receipt != receipt || !normalReceiptMatchesStage(receipt, stage) {
		return false, ErrCraftDockerNormalInputConflict
	}
	return r.claimDockerExecSend(ctx, key, revision, receipt.DockerExecReceipt, true)
}
