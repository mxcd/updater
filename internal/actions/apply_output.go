package actions

import (
	"fmt"
	"os"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
)

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

		// Group by target file for commits
		fileGroups := groupUpdatesByFile(group.Updates)
		totalCommits += len(fileGroups)

		t := table.NewWriter()
		t.SetOutputMirror(os.Stdout)
		t.AppendHeader(table.Row{"Target", "File", "Source", "Current", "→", "Latest", "Type"})

		for _, update := range group.Updates {
			displayName := update.TargetName
			if update.ItemName != "" {
				displayName = update.ItemName
			}

			t.AppendRow(table.Row{
				displayName,
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

		for _, update := range group.Updates {
			displayName := update.TargetName
			if update.ItemName != "" {
				displayName = update.ItemName
			}

			t.AppendRow(table.Row{
				displayName,
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