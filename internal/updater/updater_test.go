package updater

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareAcceptsWritableTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "yonime.exe")
	source := filepath.Join(t.TempDir(), "yonime-1.4.exe")
	writeTestFile(t, target, []byte("MZ old"))
	writeTestFile(t, source, []byte("MZ new"))

	plan, err := Prepare(target, source, t.TempDir(), "")
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if plan.Target != target {
		t.Fatalf("target = %q", plan.Target)
	}
	if plan.Source != source {
		t.Fatalf("source = %q", plan.Source)
	}
	if !strings.HasSuffix(plan.Script, ".cmd") {
		t.Fatalf("script = %q", plan.Script)
	}
}

func TestPrepareRejectsMissingSource(t *testing.T) {
	target := filepath.Join(t.TempDir(), "yonime.exe")
	writeTestFile(t, target, []byte("MZ old"))
	if _, err := Prepare(target, filepath.Join(t.TempDir(), "missing.exe"), t.TempDir(), ""); err == nil {
		t.Fatal("expected an error")
	}
}

func TestPrepareRejectsNonExeSource(t *testing.T) {
	target := filepath.Join(t.TempDir(), "yonime.exe")
	source := filepath.Join(t.TempDir(), "update.bin")
	writeTestFile(t, target, []byte("MZ old"))
	writeTestFile(t, source, []byte("plain text"))
	if _, err := Prepare(target, source, t.TempDir(), ""); err == nil {
		t.Fatal("expected an error")
	}
}

func TestCheckTargetRejectsMissingTarget(t *testing.T) {
	if err := CheckTarget(filepath.Join(t.TempDir(), "missing.exe")); err == nil {
		t.Fatal("expected an error")
	}
}

func TestHelperScriptReplacesWithoutOldBackup(t *testing.T) {
	script := helperScript()
	for _, fragment := range []string{
		`set "NEW=%TARGET%.new"`,
		`copy /y "%SOURCE%" "%NEW%"`,
		`fc /b "%SOURCE%" "%NEW%"`,
		`move /y "%NEW%" "%TARGET%"`,
		`del /f /q "%SOURCE%"`,
		`start "" "%TARGET%"`,
		`:failed`,
	} {
		if !strings.Contains(script, fragment) {
			t.Errorf("script is missing %q", fragment)
		}
	}
	if strings.Contains(script, ".old") {
		t.Error("script must not create yonime.exe.old")
	}
	if strings.Contains(script, "Обновление") {
		t.Error("script must stay ASCII-only")
	}
}
