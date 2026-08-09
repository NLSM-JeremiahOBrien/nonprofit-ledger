package api_test

import (
	"os/exec"
	"strings"
	"testing"
)

// TestNoTelemetryDependencies asserts the full module dependency graph
// contains no analytics, error-tracking, cloud-storage, or telemetry
// package — the load-bearing PLAT-04 guarantee that the app has no
// built-in path to phone home or exfiltrate data to a third party,
// verified at the module-graph level rather than merely by code review.
func TestNoTelemetryDependencies(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "./...").Output()
	if err != nil {
		t.Fatalf("go list -deps ./...: %v\noutput: %s", err, out)
	}

	lower := strings.ToLower(string(out))

	banned := []string{"analytics", "sentry", "s3", "telemetry"}
	for _, term := range banned {
		if strings.Contains(lower, term) {
			t.Errorf("module dependency graph contains banned term %q — PLAT-04 requires no telemetry/analytics/cloud-storage dependency anywhere in the module graph", term)
		}
	}
}
