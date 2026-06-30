package e2e_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestE2ERedactionScanChecksURLPathSegment(t *testing.T) {
	artifactDir := t.TempDir()
	part := "opaque-token-123456789"
	script := fmt.Sprintf(`
set -Eeuo pipefail
source ./lib/e2e.sh
configured_url="https://example.invalid/feed/opaque-token%%2D123456789"
printf 'path value: %[1]s\n' >"${E2E_ARTIFACT_DIR}/path-value-output.txt"
assert_artifacts_do_not_contain_sensitive_values "path-value" "${configured_url}"
`, part)

	result := runBash(t, artifactDir, script)
	if result.err == nil {
		t.Fatalf("redaction scan should fail for configured URL path value")
	}
	assertNoSecretLeak(t, result.stdout, part, "stdout")
	assertNoSecretLeak(t, result.stderr, part, "stderr")

	reportPath := filepath.Join(artifactDir, "path-value-redaction-scan.txt")
	report, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read redaction report: %v", err)
	}
	assertNoSecretLeak(t, string(report), part, "redaction report")
	if !strings.Contains(string(report), "path-value-output.txt") {
		t.Fatalf("expected report to identify leaking artifact path, got %q", string(report))
	}
}

func TestE2ERedactionScanPythonHelperHasValidSyntax(t *testing.T) {
	cmd := exec.Command("python3", "-m", "py_compile", "lib/redaction_scan.py")
	cmd.Dir = "."
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("python syntax check failed: %v\n%s", err, output)
	}
}
