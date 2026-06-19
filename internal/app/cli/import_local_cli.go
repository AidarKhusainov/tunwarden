package cli

import (
	"fmt"
	"io"

	"github.com/AidarKhusainov/podlaz/internal/profile"
	"github.com/AidarKhusainov/podlaz/internal/render"
)

func runLocalFileImport(path string, stdout io.Writer, opts options) error {
	content, err := profile.ReadLocalImportFile(path)
	if err != nil {
		return err
	}
	result, err := profile.ImportLocalContent(content)
	if err != nil {
		return usageError("%s", render.Redact(err.Error()))
	}
	store, err := profile.NewStore(opts.profileStorePath)
	if err != nil {
		return err
	}
	if err := store.AddProfiles(result.Profiles); err != nil {
		return profileCommandError(err)
	}
	printLocalImportResult(stdout, result)
	return nil
}

func printLocalImportResult(stdout io.Writer, result profile.LocalImportResult) {
	fmt.Fprintln(stdout, "Local import completed")
	fmt.Fprintf(stdout, "Format: %s\n", result.Format)
	fmt.Fprintf(stdout, "Inspected: %d\n", result.Inspected)
	fmt.Fprintf(stdout, "Imported: %d\n", len(result.Profiles))
	fmt.Fprintf(stdout, "Skipped: %d\n", len(result.Unsupported))
	fmt.Fprintf(stdout, "Warnings: %d\n", len(result.Warnings))
	if len(result.Profiles) > 0 {
		fmt.Fprintln(stdout, "Imported profiles:")
		for _, p := range result.Profiles {
			fmt.Fprintf(stdout, "- %s %s\n", render.Redact(p.ID), render.Redact(p.Name))
		}
	}
	if len(result.Unsupported) > 0 {
		fmt.Fprintln(stdout, "Skipped entries:")
		for _, issue := range result.Unsupported {
			fmt.Fprintf(stdout, "- entry %d: %s\n", issue.Entry, render.Redact(issue.Message))
		}
	}
	if len(result.Warnings) > 0 {
		fmt.Fprintln(stdout, "Warning details:")
		for _, warning := range result.Warnings {
			fmt.Fprintf(stdout, "- entry %d: %s\n", warning.Entry, render.Redact(warning.Message))
		}
	}
}
