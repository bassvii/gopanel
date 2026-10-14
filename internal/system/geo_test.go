// SPDX-License-Identifier: AGPL-3.0-or-later

package system

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVerifySHA256(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "test.txt")

	if err := os.WriteFile(p, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// sha256("hello\n") =
	//  5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03
	const want = "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03"

	if err := verifySHA256(p, want); err != nil {
		t.Fatalf("verify failed: %v", err)
	}

	if err := verifySHA256(p, "deadbeef"); err == nil {
		t.Fatal("expected mismatch, got nil")
	}

	if err := verifySHA256(p, ""); err == nil {
		t.Fatal("expected error for empty expected, got nil")
	}
}
