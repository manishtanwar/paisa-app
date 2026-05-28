package cmd

import (
	"fmt"
	"os"

	"github.com/ananthakumaran/paisa/internal/importer"
	"github.com/ananthakumaran/paisa/internal/model/template"
	"github.com/ananthakumaran/paisa/internal/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

var importTemplateName string
var importOutput string
var importAppend bool
var importNoPredict bool
var importListTemplates bool

var importCmd = &cobra.Command{
	Use:   "import [file]",
	Short: "Import a statement file using a template",
	Long: `Import a statement file (CSV, XLSX, XLS) using a named import template.

The template processes the file the same way the web UI does, applying
Handlebars template logic and ML-based account prediction. Output is
formatted ledger transactions written to stdout by default.

PDF files are not supported in the CLI; use 'paisa serve' for PDF imports.

Note: the 'match' helper evaluates hash patterns in sorted key order (not
insertion order). Use non-overlapping patterns for predictable behavior.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if importListTemplates {
			for _, t := range template.All() {
				fmt.Printf("%s\t(%s)\n", t.Name, t.TemplateType)
			}
			return
		}

		if len(args) == 0 {
			log.Fatal("file argument is required (or use --list-templates)")
		}
		if importTemplateName == "" {
			log.Fatal("--template is required")
		}

		var db *gorm.DB
		if !importNoPredict {
			var err error
			db, err = utils.OpenDB()
			if err != nil {
				log.Fatalf("failed to open database: %v", err)
			}
		}

		result, err := importer.Run(args[0], importTemplateName, db, importNoPredict)
		if err != nil {
			log.Fatal(err)
		}

		if importOutput != "" {
			flag := os.O_WRONLY | os.O_CREATE
			if importAppend {
				flag |= os.O_APPEND
			} else {
				flag |= os.O_TRUNC
			}
			f, err := os.OpenFile(importOutput, flag, 0644)
			if err != nil {
				log.Fatalf("failed to open output file: %v", err)
			}
			defer f.Close()
			if _, err := fmt.Fprintln(f, result); err != nil {
				log.Fatalf("failed to write output: %v", err)
			}
		} else {
			if importAppend {
				log.Warn("--append has no effect without --output")
			}
			fmt.Println(result)
		}
	},
}

func init() {
	rootCmd.AddCommand(importCmd)
	importCmd.Flags().StringVarP(&importTemplateName, "template", "t", "", "import template name (from paisa.yaml or builtin)")
	importCmd.Flags().StringVarP(&importOutput, "output", "o", "", "write output to file instead of stdout")
	importCmd.Flags().BoolVar(&importAppend, "append", false, "append to output file instead of overwriting")
	importCmd.Flags().BoolVar(&importNoPredict, "no-predict", false, "skip ML account prediction (use template defaults)")
	importCmd.Flags().BoolVar(&importListTemplates, "list-templates", false, "list available template names and exit")
}
