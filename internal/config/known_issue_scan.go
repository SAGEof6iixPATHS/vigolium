package config

import (
	"fmt"
)

// KnownIssueScanConfig holds configuration for known-issue scan (nuclei library).
type KnownIssueScanConfig struct {
	Tags          []string `yaml:"tags"`           // nuclei template tags (empty = all)
	ExcludeTags   []string `yaml:"exclude_tags"`   // tags to exclude
	Severities    []string `yaml:"severities"`     // filter severities (empty = all)
	TemplatesDir  string   `yaml:"templates_dir"`  // custom templates path
	EnrichTargets bool     `yaml:"enrich_targets"` // enrich known-issue scan targets with paths discovered in previous phases (increases coverage but can slow down scans)
}

// DefaultKnownIssueScanConfig returns default known-issue scan configuration.
//
// Severities defaults to critical+high only: at the default (balanced) intensity
// the known-issue scan focuses on high-signal findings rather than enumerating
// every info/low template, which keeps the phase within its time budget. Operators
// who want the full sweep can widen it with:
//
//	vigolium config set known_issue_scan.severities "critical,high,medium,low,info"
func DefaultKnownIssueScanConfig() *KnownIssueScanConfig {
	return &KnownIssueScanConfig{
		Severities:    []string{"critical", "high"},
		ExcludeTags:   []string{"dos"},
		EnrichTargets: true,
	}
}

// Validate checks known-issue scan configuration for errors.
func (c *KnownIssueScanConfig) Validate() error {
	validSeverities := map[string]bool{
		"critical": true, "high": true, "medium": true,
		"low": true, "info": true,
	}
	for _, s := range c.Severities {
		if !validSeverities[s] {
			return fmt.Errorf("known_issue_scan.severities: invalid severity %q", s)
		}
	}

	return nil
}
