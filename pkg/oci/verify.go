package oci

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// VerifyOptions configures a verification operation for a single skill.
type VerifyOptions struct {
	Name string // skill name (for display)
	Ref  string // digest-pinned OCI reference, e.g. ghcr.io/org/skills/foo:1.0.0@sha256:...
}

// VerifyResult holds the outcome of cosign signature and SLSA provenance checks.
type VerifyResult struct {
	Name string
	Ref  string

	// Signature verification
	SignatureVerified bool
	SignatureOutput   string // raw cosign output or error message

	// SLSA provenance verification
	SLSAVerified bool
	SLSAOutput   string // raw cosign output or error message

	// Set when cosign is not installed on the system
	CosignMissing bool

	// Set when the skill has not been installed yet (no lock entry)
	NotInstalled bool
}

// Verify runs cosign signature and SLSA provenance checks against a skill's
// OCI reference. Both checks use keyless (Sigstore) verification with
// permissive regexp matchers so they work for any publisher; callers can
// inspect the raw output to review the actual identity/issuer.
func Verify(ctx context.Context, opts VerifyOptions) VerifyResult {
	result := VerifyResult{Name: opts.Name, Ref: opts.Ref}

	if _, err := exec.LookPath("cosign"); err != nil {
		result.CosignMissing = true
		return result
	}

	result.SignatureVerified, result.SignatureOutput = runCosignVerify(ctx, opts.Ref)
	result.SLSAVerified, result.SLSAOutput = runCosignVerifyAttestation(ctx, opts.Ref)

	return result
}

// runCosignVerify runs `cosign verify` against the given reference.
func runCosignVerify(ctx context.Context, ref string) (bool, string) {
	args := []string{
		"verify",
		"--certificate-identity-regexp", ".*",
		"--certificate-oidc-issuer-regexp", ".*",
		ref,
	}
	return runCosign(ctx, args)
}

// runCosignVerifyAttestation runs `cosign verify-attestation` for SLSA provenance.
func runCosignVerifyAttestation(ctx context.Context, ref string) (bool, string) {
	args := []string{
		"verify-attestation",
		"--type", "slsaprovenance",
		"--certificate-identity-regexp", ".*",
		"--certificate-oidc-issuer-regexp", ".*",
		ref,
	}
	return runCosign(ctx, args)
}

// runCosign executes cosign with the given arguments and returns success and trimmed output.
func runCosign(ctx context.Context, args []string) (bool, string) {
	cmd := exec.CommandContext(ctx, "cosign", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	combined := strings.TrimSpace(stdout.String())
	if combined == "" {
		combined = strings.TrimSpace(stderr.String())
	}

	// Trim verbose JSON output to first meaningful line for display.
	if lines := strings.SplitN(combined, "\n", 2); len(lines) > 0 {
		combined = strings.TrimSpace(lines[0])
	}

	if combined == "" && err != nil {
		combined = fmt.Sprintf("cosign exited with: %v", err)
	}

	return err == nil, combined
}
