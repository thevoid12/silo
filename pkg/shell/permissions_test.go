package shell

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadPermissions_roundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "allowed_permissions.md")

	// Save three commands
	for _, cmd := range []string{"echo", "tee", "curl"} {
		if err := SavePermission(path, cmd); err != nil {
			t.Fatalf("SavePermission(%q): %v", cmd, err)
		}
	}

	cmds, err := loadPermissions(path)
	if err != nil {
		t.Fatalf("loadPermissions: %v", err)
	}
	if len(cmds) != 3 {
		t.Fatalf("expected 3 commands, got %d: %v", len(cmds), cmds)
	}
	want := map[string]bool{"echo": true, "tee": true, "curl": true}
	for _, c := range cmds {
		if !want[c] {
			t.Errorf("unexpected command %q in loaded permissions", c)
		}
	}
}

func TestLoadPermissions_missingFile(t *testing.T) {
	cmds, err := loadPermissions("/nonexistent/path/allowed_permissions.md")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got: %v", err)
	}
	if len(cmds) != 0 {
		t.Fatalf("expected empty slice, got: %v", cmds)
	}
}

func TestLoadPermissions_skipsHeaderLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "allowed_permissions.md")
	content := "# Silo Allowed Permissions\n\nCommands approved for automatic execution:\n\n- sh\n- git\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cmds, err := loadPermissions(path)
	if err != nil {
		t.Fatalf("loadPermissions: %v", err)
	}
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands (not header lines), got %d: %v", len(cmds), cmds)
	}
}

func TestSavePermission_appendsOnSubsequentCalls(t *testing.T) {
	path := filepath.Join(t.TempDir(), "allowed_permissions.md")

	if err := SavePermission(path, "wget"); err != nil {
		t.Fatal(err)
	}
	if err := SavePermission(path, "python3"); err != nil {
		t.Fatal(err)
	}

	cmds, err := loadPermissions(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands after two saves, got %d: %v", len(cmds), cmds)
	}
}
