package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRunCLICompletionGeneratesPlzAliasSupport(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "bash",
			args: []string{"completion", "bash"},
			want: []string{"complete -o default -F _podlaz podlaz plz"},
		},
		{
			name: "zsh",
			args: []string{"completion", "zsh"},
			want: []string{"#compdef podlaz plz"},
		},
		{
			name: "fish",
			args: []string{"completion", "fish"},
			want: []string{"complete -c plz -f", "complete -c plz -n '__fish_podlaz_using_command plan' -l mode"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := run(context.Background(), tt.args, &out); err != nil {
				t.Fatalf("completion command failed: %v", err)
			}
			got := out.String()
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Fatalf("expected completion output to contain %q, got %q", want, got)
				}
			}
		})
	}
}

func TestRunCLICompletionRuntimeAcceptsPlzCommandName(t *testing.T) {
	got := runCompletionRuntime(t, options{}, bashCompleteArgs(1, "plz", "")...)
	for _, want := range []string{"connect", "completion", "profile", "subscription"} {
		assertContainsLine(t, got, want)
	}
}
