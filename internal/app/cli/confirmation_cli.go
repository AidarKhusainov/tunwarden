package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

func confirmationReader(opts options) io.Reader {
	if opts.stdin != nil {
		return opts.stdin
	}
	return os.Stdin
}

func confirmDefaultYes(stdout io.Writer, reader io.Reader, prompt, readContext, cancelMessage string) error {
	input := bufio.NewReader(reader)
	for {
		if _, err := fmt.Fprintf(stdout, "%s [Y/n]: ", prompt); err != nil {
			return err
		}
		line, err := input.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("read %s confirmation: %w", readContext, err)
		}
		if errors.Is(err, io.EOF) && line == "" {
			return exitError{code: 1, err: errors.New(cancelMessage)}
		}

		confirmed, ok := parseDefaultYesConfirmation(line)
		if ok {
			if confirmed {
				return nil
			}
			return exitError{code: 1, err: errors.New(cancelMessage)}
		}

		if _, writeErr := fmt.Fprintln(stdout, "Please answer y or n."); writeErr != nil {
			return writeErr
		}
		if errors.Is(err, io.EOF) {
			return exitError{code: 1, err: errors.New(cancelMessage)}
		}
	}
}

func parseDefaultYesConfirmation(input string) (confirmed bool, valid bool) {
	normalized := strings.ToLower(strings.TrimSpace(input))
	switch normalized {
	case "y", "yes":
		return true, true
	case "n", "no":
		return false, true
	case "":
		return strings.Contains(input, "\n"), strings.Contains(input, "\n")
	default:
		return false, false
	}
}
