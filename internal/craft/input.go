package craft

import (
	"fmt"
	"strings"
)

// Staging limits for authorized material entering a Craft workspace. The
// service enforces them twice: against the declared manifest before any
// bytes are read, and against the actually-read bytes after the bounded
// read, so a lying declaration cannot smuggle an over-budget file past the
// first gate.
const (
	// MaxInputBytes caps one staged file (20 MiB).
	MaxInputBytes = 20 << 20
	// MaxInputsPerRound caps how many files one delegation may stage.
	MaxInputsPerRound = 20
	// MaxTotalInputBytes caps the summed size of one round of staged files
	// (100 MiB).
	MaxTotalInputBytes = 100 << 20
)

// InputDir is the workspace-relative directory that holds staged material.
// Inside the sandbox the workspace root is /workspace, so staged content
// lands under /workspace/inputs, which the runtime image mounts read-only.
const InputDir = "inputs"

// ValidateInputName accepts exactly one canonical path element: non-empty,
// not a dot entry, and free of separators and NUL. Everything that could
// escape the inputs directory — traversal, absolute paths, nested paths,
// backslashes — is rejected before delegation.
func ValidateInputName(name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\\x00") {
		return fmt.Errorf("%w: input name %q", ErrInvalidInput, name)
	}
	return nil
}

// ValidSHA256 reports whether digest is exactly 64 lowercase hex characters,
// the canonical form produced by every staging and artifact path.
func ValidSHA256(digest string) bool {
	if len(digest) != 64 {
		return false
	}
	for i := 0; i < len(digest); i++ {
		c := digest[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
			continue
		}
		return false
	}
	return true
}

// InputPath returns the canonical workspace-relative staging path for one
// input: inputs/<sha256>/<name>. Content addressing by digest means two
// rounds staging different content under the same file name never collide,
// and the reference manifest keeps the original source ref for citation.
// The name must already be canonical (ValidateInputName) and the digest must
// be a valid lowercase SHA-256; callers stage only after both the declared
// and the actual digest were verified equal.
func InputPath(sha256, name string) (string, error) {
	if !ValidSHA256(sha256) {
		return "", fmt.Errorf("%w: input digest %q", ErrInvalidInput, sha256)
	}
	if err := ValidateInputName(name); err != nil {
		return "", err
	}
	return InputDir + "/" + sha256 + "/" + name, nil
}

// ValidateInputManifest enforces every pre-read rule on one round of
// declared inputs: count cap, per-file size cap, summed size cap, non-empty
// ref, canonical name and well-formed digest. It runs before any authorized
// read so over-budget requests are refused without loading a byte.
func ValidateInputManifest(inputs []Input) error {
	if len(inputs) > MaxInputsPerRound {
		return fmt.Errorf("%w: %d inputs exceed the per-round cap %d",
			ErrInvalidInput, len(inputs), MaxInputsPerRound)
	}
	var total int64
	for _, in := range inputs {
		if strings.TrimSpace(in.Ref) == "" {
			return fmt.Errorf("%w: input %q has no source ref", ErrInvalidInput, in.Name)
		}
		if err := ValidateInputName(in.Name); err != nil {
			return err
		}
		if !ValidSHA256(in.SHA256) {
			return fmt.Errorf("%w: input %q has malformed digest", ErrInvalidInput, in.Name)
		}
		if in.Bytes <= 0 {
			return fmt.Errorf("%w: input %q has no declared size", ErrInvalidInput, in.Name)
		}
		if in.Bytes > MaxInputBytes {
			return fmt.Errorf("%w: input %q declares %d bytes over the %d cap",
				ErrInvalidInput, in.Name, in.Bytes, MaxInputBytes)
		}
		total += in.Bytes
		if total > MaxTotalInputBytes {
			return fmt.Errorf("%w: inputs declare %d bytes over the %d total cap",
				ErrInvalidInput, total, MaxTotalInputBytes)
		}
	}
	return nil
}
