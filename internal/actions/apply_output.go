package actions

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/mxcd/updater/internal/compare"
)

// splitByWildcard separates updates into sorted wildcard groups and non-wildcard updates.
func splitByWildcard(updates []*UpdateItem) (patterns []string, wildcardGroups map[string][]*UpdateItem, nonWildcard []*UpdateItem) {
	wildcardGroups = make(map[string][]*UpdateItem)
	nonWildcard = make([]*UpdateItem, 0)

	for _, update := range updates {
		if update.IsWildcardMatch && update.WildcardPattern != "" {
			wildcardGroups[update.WildcardPattern] = append(wildcardGroups[update.WildcardPattern], update)
		} else {
			nonWildcard = append(nonWildcard, update)
		}
	}

	patterns = make([]string, 0, len(wildcardGroups))
	for p := range wildcardGroups {
		patterns = append(patterns, p)
	}
	sort.Strings(patterns)

	return patterns, wildcardGroups, nonWildcard
}

// formatUpdateType adds an emoji indicator to the update type for PR bodies.
func formatUpdateType(ut compare.UpdateType) string {
	s := string(ut)
	switch ut {
	case compare.UpdateTypeMajor:
		return "🔴 " + s
	case compare.UpdateTypeMinor:
		return "🟡 " + s
	case compare.UpdateTypePatch:
		return "🟢 " + s
	default:
		return s
	}
}

// displayName returns the best display name for an update item.
func displayName(update *UpdateItem) string {
	if update.ItemName != "" {
		return update.ItemName
	}
	return update.TargetName
}

// outputDryRunPlan outputs the plan in dry-run mode
func outputDryRunPlan(groups []*PatchGroup) {
	fmt.Println("\n🔍 DRY RUN - Apply Plan")
	fmt.Println("========================")

	totalCommits := 0
	totalPRs := len(groups)

	for i, group := range groups {
		fmt.Printf("📦 Patch Group %d/%d: %s\n", i+1, len(groups), group.Name)
		if len(group.Labels) > 0 {
			fmt.Printf("   Labels: %s\n", strings.Join(group.Labels, ", "))
		}
		fmt.Printf("   Updates: %d\n\n", len(group.Updates))

		fileGroups := groupUpdatesByFile(group.Updates)
		totalCommits += len(fileGroups)

		t := table.NewWriter()
		t.SetOutputMirror(os.Stdout)
		t.AppendHeader(table.Row{"Target", "File", "Source", "Current", "→", "Latest", "Type"})

		patterns, wildcardGroups, nonWildcardUpdates := splitByWildcard(group.Updates)

		for _, pattern := range patterns {
			groupUpdates := wildcardGroups[pattern]
			t.AppendRow(table.Row{
				fmt.Sprintf("📦 Wildcard: %s", pattern),
				fmt.Sprintf("(%d files)", len(groupUpdates)),
				"", "", "", "", "",
			})
			for _, update := range groupUpdates {
				t.AppendRow(table.Row{
					"  ↳ " + displayName(update),
					update.TargetFile,
					update.SourceName,
					update.CurrentVersion,
					"→",
					update.LatestVersion,
					update.UpdateType,
				})
			}
			t.AppendSeparator()
		}

		for _, update := range nonWildcardUpdates {
			t.AppendRow(table.Row{
				displayName(update),
				update.TargetFile,
				update.SourceName,
				update.CurrentVersion,
				"→",
				update.LatestVersion,
				update.UpdateType,
			})
		}

		t.SetStyle(table.StyleRounded)
		t.Render()
		fmt.Println()

		fmt.Printf("   📝 Would create: %d commit(s) in %d file(s)\n", len(fileGroups), len(fileGroups))
		fmt.Printf("   🔀 Would create: 1 pull request\n")
		if len(group.Labels) > 0 {
			fmt.Printf("   🏷️  PR labels: %s\n", strings.Join(group.Labels, ", "))
		}
		fmt.Println()
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("📊 Summary:\n")
	fmt.Printf("   • Total patch groups: %d\n", totalPRs)
	fmt.Printf("   • Total commits: %d\n", totalCommits)
	fmt.Printf("   • Total pull requests: %d\n", totalPRs)
	fmt.Println()
	fmt.Println("💡 This is a dry run. Use 'apply' without --dry-run to execute.")
}

// outputLocalPlan outputs the plan for local-only mode (no git operations)
func outputLocalPlan(updates []*UpdateItem) {
	fmt.Println("\n📂 Local Apply Plan")
	fmt.Println("====================")
	fmt.Printf("   Updates: %d\n\n", len(updates))

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Target", "File", "Current", "→", "Latest", "Type"})

	patterns, wildcardGroups, nonWildcardUpdates := splitByWildcard(updates)

	for _, pattern := range patterns {
		groupUpdates := wildcardGroups[pattern]
		t.AppendRow(table.Row{
			fmt.Sprintf("📦 Wildcard: %s", pattern),
			fmt.Sprintf("(%d files)", len(groupUpdates)),
			"", "", "", "",
		})
		for _, update := range groupUpdates {
			t.AppendRow(table.Row{
				"  ↳ " + displayName(update),
				update.TargetFile,
				update.CurrentVersion,
				"→",
				update.LatestVersion,
				update.UpdateType,
			})
		}
		t.AppendSeparator()
	}

	for _, update := range nonWildcardUpdates {
		t.AppendRow(table.Row{
			displayName(update),
			update.TargetFile,
			update.CurrentVersion,
			"→",
			update.LatestVersion,
			update.UpdateType,
		})
	}

	t.SetStyle(table.StyleRounded)
	t.Render()
	fmt.Println()
}

// outputApplyPlan outputs the plan for actual execution
func outputApplyPlan(groups []*PatchGroup) {
	fmt.Println("\n🚀 Apply Plan")
	fmt.Println("=============")

	for i, group := range groups {
		fmt.Printf("📦 Patch Group %d/%d: %s\n", i+1, len(groups), group.Name)
		if len(group.Labels) > 0 {
			fmt.Printf("   Labels: %s\n", strings.Join(group.Labels, ", "))
		}

		t := table.NewWriter()
		t.SetOutputMirror(os.Stdout)
		t.AppendHeader(table.Row{"Target", "File", "Current", "→", "Latest", "Type"})

		patterns, wildcardGroups, nonWildcardUpdates := splitByWildcard(group.Updates)

		for _, pattern := range patterns {
			groupUpdates := wildcardGroups[pattern]
			t.AppendRow(table.Row{
				fmt.Sprintf("📦 Wildcard: %s", pattern),
				fmt.Sprintf("(%d files)", len(groupUpdates)),
				"", "", "", "",
			})
			for _, update := range groupUpdates {
				t.AppendRow(table.Row{
					"  ↳ " + displayName(update),
					update.TargetFile,
					update.CurrentVersion,
					"→",
					update.LatestVersion,
					update.UpdateType,
				})
			}
			t.AppendSeparator()
		}

		for _, update := range nonWildcardUpdates {
			t.AppendRow(table.Row{
				displayName(update),
				update.TargetFile,
				update.CurrentVersion,
				"→",
				update.LatestVersion,
				update.UpdateType,
			})
		}

		t.SetStyle(table.StyleRounded)
		t.Render()
		fmt.Println()
	}
}
