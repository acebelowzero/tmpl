package sops

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
)

// Decryptor abstracts secret decryption to facilitate testing.
type Decryptor interface {
	Decrypt(ctx context.Context, data []byte) ([]byte, error)
	DecryptFile(ctx context.Context, path string) ([]byte, error)
}

type execDecryptor struct{}

// New constructs a default Decryptor implementation backed by the sops CLI.
func New() (Decryptor, error) {
	if _, err := exec.LookPath("sops"); err != nil {
		return nil, fmt.Errorf("sops binary not found: %w", err)
	}
	return &execDecryptor{}, nil
}

func (d *execDecryptor) Decrypt(ctx context.Context, data []byte) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "sops", "-d", "/dev/stdin")
	cmd.Stdin = bytes.NewReader(data)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("sops decrypt: %w: %s", err, string(out))
	}
	return out, nil
}

func (d *execDecryptor) DecryptFile(ctx context.Context, path string) ([]byte, error) {
	if path == "" {
		return nil, errors.New("missing path for sops decrypt")
	}
	cmd := exec.CommandContext(ctx, "sops", "-d", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("sops decrypt file %s: %w: %s", path, err, string(out))
	}
	return out, nil
}
