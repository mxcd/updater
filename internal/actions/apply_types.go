package actions

import "github.com/mxcd/updater/internal/compare"

// ApplyOptions represents options for the apply command
type ApplyOptions struct {
	ConfigPath   string
	OutputFormat string
	DryRun       bool
	Local        bool
	Limit        int
	Only         string
}

// PatchGroup represents a group of updates that should be applied together
type PatchGroup struct {
	Name    string
	Updates []*UpdateItem
	Labels  []string
}

// UpdateItem represents a single update to be applied
type UpdateItem struct {
	TargetName      string
	TargetFile      string
	ItemName        string
	SourceName      string
	CurrentVersion  string
	LatestVersion   string
	UpdateType      compare.UpdateType
	PatchGroup      string
	Labels          []string
	WildcardPattern string // Original wildcard pattern if this target was expanded
	IsWildcardMatch bool   // Flag indicating if this came from a wildcard expansion
}
