package cmd

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/internal/report"
)

// reportOutput is the file path the HTML report is written to.
var reportOutput string

// reportTitle overrides the document title.
var reportTitle string

// reportTopTags caps the tag histogram length.
var reportTopTags int

// reportCmd groups vault reporting commands.
var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Render an HTML summary of the vault.",
}

// reportHTMLCmd writes the HTML report to the chosen file.
var reportHTMLCmd = &cobra.Command{
	Use:   "html",
	Short: "Render a single-file HTML report to disk.",
	RunE:  runReportHTML,
}

func init() {
	reportHTMLCmd.Flags().StringVarP(&reportOutput, "out", "o", "midden-report.html", "Output file path.")
	reportHTMLCmd.Flags().StringVar(&reportTitle, "title", "midden", "Title shown in the report.")
	reportHTMLCmd.Flags().IntVar(&reportTopTags, "top-tags", 20, "Maximum number of tag bars to render.")
	reportCmd.AddCommand(reportHTMLCmd)
	rootCmd.AddCommand(reportCmd)
}

// runReportHTML builds and writes the HTML report to disk.
func runReportHTML(cmd *cobra.Command, _ []string) error {
	v, err := openVault()
	if err != nil {
		return err
	}
	data, err := report.Build(reportTitle, v, time.Now(), reportTopTags)
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("build report: %w", err))
	}
	f, err := os.Create(reportOutput)
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("create %s: %w", reportOutput, err))
	}
	defer f.Close()
	if err := report.Render(f, data); err != nil {
		return errors.Join(ErrVault, fmt.Errorf("render report: %w", err))
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Wrote report to %s\n", reportOutput)
	return nil
}
