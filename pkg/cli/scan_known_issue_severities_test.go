package cli

import "testing"

// TestShouldWidenKnownIssueScanSeverities pins the rule that running ONLY the
// known-issue-scan phase widens its severity filter to all levels — unless the
// user pinned severities, another phase is also selected, or the configured set
// already covers everything.
func TestShouldWidenKnownIssueScanSeverities(t *testing.T) {
	tests := []struct {
		name               string
		onlyPhase          string
		severitiesExplicit bool
		configured         []string
		want               bool
	}{
		{
			name:       "isolated known-issue-scan with balanced default widens",
			onlyPhase:  "known-issue-scan",
			configured: []string{"critical", "high"},
			want:       true,
		},
		{
			name:               "explicit severities are respected (no widen)",
			onlyPhase:          "known-issue-scan",
			severitiesExplicit: true,
			configured:         []string{"critical", "high"},
			want:               false,
		},
		{
			name:       "multi-phase run does not widen",
			onlyPhase:  "known-issue-scan,discovery",
			configured: []string{"critical", "high"},
			want:       false,
		},
		{
			name:       "full pipeline (no --only) does not widen",
			onlyPhase:  "",
			configured: []string{"critical", "high"},
			want:       false,
		},
		{
			name:       "already empty (= all) does not widen",
			onlyPhase:  "known-issue-scan",
			configured: nil,
			want:       false,
		},
		{
			name:       "already covers all five does not widen",
			onlyPhase:  "known-issue-scan",
			configured: []string{"info", "low", "medium", "high", "critical"},
			want:       false,
		},
		{
			name:       "case/space-insensitive coverage does not widen",
			onlyPhase:  "known-issue-scan",
			configured: []string{" Critical ", "HIGH", "Medium", "low", "INFO"},
			want:       false,
		},
		{
			name:       "partial non-default set still widens",
			onlyPhase:  "known-issue-scan",
			configured: []string{"medium"},
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldWidenKnownIssueScanSeverities(tt.onlyPhase, tt.severitiesExplicit, tt.configured)
			if got != tt.want {
				t.Fatalf("shouldWidenKnownIssueScanSeverities(%q, %v, %v) = %v, want %v",
					tt.onlyPhase, tt.severitiesExplicit, tt.configured, got, tt.want)
			}
		})
	}
}
