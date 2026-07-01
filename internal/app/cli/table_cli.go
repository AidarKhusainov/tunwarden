package cli

import (
	"fmt"
	"io"
	"text/tabwriter"
)

func writeTable(w io.Writer, header []string, rows [][]string) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if err := writeTableRow(tw, header); err != nil {
		return err
	}
	for _, row := range rows {
		if err := writeTableRow(tw, row); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func writeTableRow(w io.Writer, columns []string) error {
	for i, column := range columns {
		if i > 0 {
			if _, err := fmt.Fprint(w, "\t"); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprint(w, column); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(w)
	return err
}
