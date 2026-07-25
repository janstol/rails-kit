package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestLocalesRejectsExtraArgs(t *testing.T) {
	if err := localesCmd.Args(localesCmd, []string{"en.views", "extra"}); err == nil {
		t.Fatal("expected argument validation error")
	}
}

func TestFixturesRejectsExtraArgs(t *testing.T) {
	if err := fixturesCmd.Args(fixturesCmd, []string{"users", "extra"}); err == nil {
		t.Fatal("expected argument validation error")
	}
}

func TestCompletionRejectsExtraArgs(t *testing.T) {
	tests := []struct {
		name string
		cmd  *cobra.Command
	}{
		{name: "bash", cmd: completionBashCmd},
		{name: "zsh", cmd: completionZshCmd},
		{name: "fish", cmd: completionFishCmd},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.cmd.Args(tt.cmd, []string{"extra"}); err == nil {
				t.Fatal("expected argument validation error")
			}
		})
	}
}

func TestAboutRejectsExtraArgs(t *testing.T) {
	if err := aboutCmd.Args(aboutCmd, []string{"extra"}); err == nil {
		t.Fatal("expected argument validation error")
	}
}
