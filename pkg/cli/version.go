package cli

import (
	"encoding/json"
	"fmt"
	"runtime"

	"silo/version"

	"github.com/spf13/cobra"
)

var (
	// These are set via ldflags at build time
	commit    = "none"
	buildDate = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  "Print the version, commit hash, build date, and platform information.",
	Run: func(cmd *cobra.Command, args []string) {
		if IsJSON() {
			printVersionJSON()
		} else {
			printVersionText()
		}
	},
}

func printVersionText() {
	fmt.Printf("\n  silo %s (%s %s)\n", version.VERSION, commit, buildDate)
	fmt.Printf("  Go:        %s\n", runtime.Version())
	fmt.Printf("  Platform:  %s/%s\n\n", runtime.GOOS, runtime.GOARCH)
}

func printVersionJSON() {
	info := map[string]string{
		"version":    version.VERSION,
		"commit":     commit,
		"build_date": buildDate,
		"go_version": runtime.Version(),
		"platform":   fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
	out, _ := json.MarshalIndent(info, "", "  ")
	fmt.Println(string(out))
}
