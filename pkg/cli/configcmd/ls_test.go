package configcmd

import "testing"

func TestKeyMatches(t *testing.T) {
	keys := []string{
		"known_issue_scan.enrich_targets",
		"scanning_pace.known_issue_scan.concurrency",
		"scanning_strategy.balanced.known_issue_scan",
		"storage.path",
		"server.port",
	}

	tests := []struct {
		name   string
		filter string
		want   []string
	}{
		{
			name:   "empty matches all",
			filter: "",
			want:   keys,
		},
		{
			name:   "substring",
			filter: "known",
			want: []string{
				"known_issue_scan.enrich_targets",
				"scanning_pace.known_issue_scan.concurrency",
				"scanning_strategy.balanced.known_issue_scan",
			},
		},
		{
			name:   "glob prefix matches same keys as substring",
			filter: "kno*",
			want: []string{
				"known_issue_scan.enrich_targets",
				"scanning_pace.known_issue_scan.concurrency",
				"scanning_strategy.balanced.known_issue_scan",
			},
		},
		{
			name:   "glob single-char wildcard",
			filter: "serv?r.port",
			want:   []string{"server.port"},
		},
		{
			name:   "glob across dots on full key",
			filter: "scanning*known*",
			want: []string{
				"scanning_pace.known_issue_scan.concurrency",
				"scanning_strategy.balanced.known_issue_scan",
			},
		},
		{
			name:   "glob with no match",
			filter: "zzz*",
			want:   nil,
		},
		{
			name:   "fuzzy subsequence still works",
			filter: "store", // s-t-o-r-...-e inside "storage"
			want:   []string{"storage.path"},
		},
		{
			name:   "malformed glob falls back to fuzzy substring",
			filter: "stora[ge", // bad pattern; substring "stora" not present, but kept harmless
			want:   nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, k := range keys {
				if keyMatches(k, tc.filter) {
					got = append(got, k)
				}
			}
			if !equalStrings(got, tc.want) {
				t.Errorf("keyMatches filter=%q\n got: %v\nwant: %v", tc.filter, got, tc.want)
			}
		})
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
