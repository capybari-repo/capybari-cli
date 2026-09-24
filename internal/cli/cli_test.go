package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/capybari/capybari-core/report"
	"github.com/capybari/capybari-schemas"
)

const fixtures = "../../../capybari-fixtures"

func run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	app := &App{Build: Build{Version: "0.0.0-test"}, Stdout: &out, Stderr: &errb, Env: func(k string) string {
		if k == "NO_COLOR" {
			return "1"
		}
		return ""
	}}
	code := app.Run(context.Background(), args)
	return code, out.String(), errb.String()
}

func TestAnalyzeOfflineWritesValidReports(t *testing.T) {
	out := t.TempDir()
	code, _, stderr := run(t, "analyze", filepath.Join(fixtures, "node-express-legacy"), "--offline", "--no-cache", "--out", out, "--format", "json,md,html,sarif")
	if code != ExitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	for _, f := range []string{"report.json", "report.md", "report.html", "report.sarif", "sbom.cdx.json"} {
		if _, err := os.Stat(filepath.Join(out, f)); err != nil {
			t.Fatalf("missing %s: %v", f, err)
		}
	}
	b, _ := os.ReadFile(filepath.Join(out, "report.json"))
	if err := schemas.Validate("report.schema.json", b); err != nil {
		t.Fatalf("report invalid: %v", err)
	}
	var r report.Report
	json.Unmarshal(b, &r)
	status := map[string]string{}
	for _, c := range r.Capabilities {
		status[c.ID] = c.Status
	}
	for id, want := range map[string]string{
		"inventory": "ok", "fingerprint": "ok", "tech-detect": "ok", "dependencies": "ok", "secrets": "ok",
		"code-health": "ok", "architecture": "ok", "vulns": "skipped",
	} {
		if status[id] != want {
			t.Errorf("%s: %q, want %q", id, status[id], want)
		}
	}
	if r.DataBoundary.LeftMachine || !r.Scan.Mode.Offline {
		t.Fatal("offline scan must not leave the machine")
	}
	var sawNetworkRec bool
	for _, rec := range r.Recommendations {
		if rec.Kind == report.RecNetwork && rec.Capability == "vulns" {
			sawNetworkRec = true
		}
	}
	if !sawNetworkRec {
		t.Error("offline scan should recommend running vulns online")
	}
	if len(r.Artifacts) != 1 || r.Artifacts[0].Path != "sbom.cdx.json" {
		t.Errorf("artifacts: %+v", r.Artifacts)
	}
	if !strings.Contains(stderr, "Node.js 12 is end-of-life") {
		t.Errorf("terminal summary missing top finding:\n%s", stderr)
	}
}

func TestFailOnAndBaseline(t *testing.T) {
	dir := filepath.Join(fixtures, "node-express-legacy")
	out := t.TempDir()
	code, _, _ := run(t, "analyze", dir, "--offline", "--no-cache", "--quiet", "--out", out, "--fail-on", "high")
	if code != ExitPolicy {
		t.Fatalf("expected policy exit, got %d", code)
	}
	base := filepath.Join(out, "report.json")
	code, _, stderr := run(t, "analyze", dir, "--offline", "--no-cache", "--quiet", "--out", t.TempDir(), "--fail-on", "high", "--baseline", base)
	if code != ExitOK {
		t.Fatalf("no new findings vs baseline, expected exit 0, got %d: %s", code, stderr)
	}
}

func TestStdoutSarifAndPlan(t *testing.T) {
	code, out, stderr := run(t, "analyze", filepath.Join(fixtures, "go-service"), "--offline", "--no-cache", "--stdout", "--format", "sarif")
	if code != ExitOK || !strings.Contains(out, `"version": "2.1.0"`) {
		t.Fatalf("sarif stdout: %d %s %s", code, out, stderr)
	}
	code, out, _ = run(t, "plan", "https://example.com", "--offline")
	if code != ExitOK || !strings.Contains(out, "web-snapshot") || !strings.Contains(out, "will be skipped: offline") {
		t.Fatalf("plan: %d %s", code, out)
	}
	code, out, _ = run(t, "plan", filepath.Join(fixtures, "go-service"), "--only", "vulns")
	if code != ExitOK || !strings.Contains(out, "dependencies") || strings.Contains(out, "secrets") {
		t.Fatalf("plan --only vulns should pull in dependencies only: %s", out)
	}
}

func TestUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{},
		{"nope"},
		{"analyze"},
		{"analyze", ".", "--format", "pdf"},
		{"analyze", ".", "--fail-on", "urgent"},
		{"analyze", ".", "--only", "not-a-capability", "--no-cache", "--quiet"},
	} {
		if code, _, _ := run(t, args...); code == ExitOK {
			t.Errorf("%v: expected failure", args)
		}
	}
	if code, out, _ := run(t, "capabilities"); code != ExitOK || !strings.Contains(out, "secrets") {
		t.Fatalf("capabilities: %s", out)
	}
	if code, out, _ := run(t, "version"); code != ExitOK || !strings.Contains(out, "0.0.0-test") {
		t.Fatalf("version: %s", out)
	}
}

func TestValidateCommand(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(bad, []byte(`{"schema_version":"0.1"}`), 0o644)
	if code, _, _ := run(t, "validate", bad); code != ExitError {
		t.Fatalf("invalid report accepted")
	}
}
