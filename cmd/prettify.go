package cmd

import (
	"fmt"
	"path/filepath"

	"os"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/ledger"
	"github.com/bmatcuk/doublestar/v4"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var prettifyCmd = &cobra.Command{
	Use:   "prettify",
	Short: "Format all ledger files",
	Long:  "Format all ledger files in the journal directory using the same rules as the editor's Prettify button.",
	Run: func(cmd *cobra.Command, args []string) {
		journalPath := config.GetJournalPath()
		dir := filepath.Dir(journalPath)
		ext := filepath.Ext(journalPath)

		pattern := dir + "/**/*" + ext
		paths, err := doublestar.FilepathGlob(pattern)
		if err != nil {
			log.Fatalf("Failed to glob ledger files: %v", err)
		}

		if len(paths) == 0 {
			fmt.Println("No ledger files found.")
			return
		}

		changed := 0
		for _, path := range paths {
			original, err := os.ReadFile(path)
			if err != nil {
				log.Warnf("Skipping %s: %v", path, err)
				continue
			}

			formatted := ledger.FormatContent(string(original))
			if formatted == string(original) {
				continue
			}

			if err := os.WriteFile(path, []byte(formatted), 0644); err != nil {
				log.Warnf("Failed to write %s: %v", path, err)
				continue
			}

			rel, _ := filepath.Rel(dir, path)
			fmt.Printf("Formatted: %s\n", rel)
			changed++
		}

		if changed == 0 {
			fmt.Println("All files already formatted.")
		} else {
			fmt.Printf("Done. Formatted %d file(s).\n", changed)
		}
	},
}

func init() {
	rootCmd.AddCommand(prettifyCmd)
}
