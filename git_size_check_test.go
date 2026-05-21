package binest

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitSizeCheckFailsHistoricalLargeBlob(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	scriptPath, err := filepath.Abs(filepath.Join("scripts", "git-size-check.sh"))
	if err != nil {
		t.Fatalf("resolve git-size-check path: %v", err)
	}

	repoDir := t.TempDir()
	runGit(t, repoDir, "init", "-q")
	runGit(t, repoDir, "config", "user.email", "binest@example.invalid")
	runGit(t, repoDir, "config", "user.name", "binest test")

	if err := os.WriteFile(filepath.Join(repoDir, "keep.txt"), []byte("keep\n"), 0600); err != nil {
		t.Fatalf("write keep.txt: %v", err)
	}
	runGit(t, repoDir, "add", "keep.txt")
	runGit(t, repoDir, "commit", "-qm", "base")
	base := strings.TrimSpace(runGit(t, repoDir, "rev-parse", "HEAD"))

	large := bytes.Repeat([]byte("x"), 2048)
	if err := os.WriteFile(filepath.Join(repoDir, "large.dat"), large, 0600); err != nil {
		t.Fatalf("write large.dat: %v", err)
	}
	runGit(t, repoDir, "add", "large.dat")
	runGit(t, repoDir, "commit", "-qm", "add large blob")
	runGit(t, repoDir, "rm", "-q", "large.dat")
	runGit(t, repoDir, "commit", "-qm", "remove large blob")

	cmd := exec.Command(scriptPath)
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(),
		"BINEST_GIT_SIZE_BASE="+base,
		"BINEST_MAX_TRACKED_BYTES=1024",
	)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("git-size-check succeeded, want failure; output:\n%s", output)
	}
	if !strings.Contains(string(output), "historical blob exceeds 1024 bytes") {
		t.Fatalf("git-size-check output missing historical blob failure:\n%s", output)
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
	return string(output)
}
