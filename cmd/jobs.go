package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/janstol/rails-kit/internal/config"
	"github.com/janstol/rails-kit/internal/jobs"
)

type jobsListJSON struct {
	Jobs []string `json:"jobs"`
}

type jobsJSON struct {
	ClassName   string   `json:"class_name"`
	ParentClass string   `json:"parent_class,omitempty"`
	RelPath     string   `json:"rel_path"`
	Concerns    []string `json:"concerns,omitempty"`
	Queue       string   `json:"queue,omitempty"`
	RetryOn     []string `json:"retry_on,omitempty"`
	DiscardOn   []string `json:"discard_on,omitempty"`
	Methods     []string `json:"methods,omitempty"`
}

var jobsCmd = &cobra.Command{
	Use:   "jobs [name]",
	Short: "List or inspect Rails ActiveJob jobs",
	Long: `Show a structural summary of a Rails ActiveJob file.

With no arguments, lists available jobs.
With a job name, extracts its queue, retry_on/discard_on handlers, included
concerns, and public methods (notably perform).

The name can be a resource name (sync_user, admin/export), the job file's
basename (sync_user_job), a CamelCase class name (SyncUserJob,
Admin::ExportJob), or a file path ending in .rb.

Only the job's own file is parsed -- queue, retry, and discard declarations
inherited from a superclass (ApplicationJob, etc.) are not resolved.
See parent_class for where to look next.`,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeJobNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, cfg, err := loadConfig()
		if err != nil {
			return err
		}

		if len(args) == 0 {
			return runJobsList(cmd, root, cfg)
		}
		return runJobsDetail(cmd, root, cfg, args[0])
	},
}

func runJobsList(cmd *cobra.Command, root string, cfg config.Config) error {
	names, err := jobs.ListNames(root, cfg.JobsPath)
	if err != nil {
		return fmt.Errorf("listing jobs: %w", err)
	}

	if jsonFlag {
		if names == nil {
			names = []string{}
		}
		return printJSON(cmd, jobsListJSON{Jobs: names})
	}

	if len(names) == 0 {
		fmt.Println("No jobs found.")
		return nil
	}
	for _, n := range names {
		fmt.Println(n)
	}
	return nil
}

func runJobsDetail(cmd *cobra.Command, root string, cfg config.Config, input string) error {
	path, err := jobs.Resolve(root, cfg.JobsPath, input)
	if err != nil {
		return coded(codeNotFound, err)
	}

	summary, err := jobs.Parse(path, root, cfg.JobsPath)
	if err != nil {
		return fmt.Errorf("parsing job: %w", err)
	}
	for _, diagnostic := range summary.ParseErrors {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %s:%d: %s\n", path, diagnostic.Line, diagnostic.Message)
	}

	if jsonFlag {
		out := jobsJSON{
			ClassName:   summary.ClassName,
			ParentClass: summary.ParentClass,
			RelPath:     summary.RelPath,
			Concerns:    trimEntries(summary.Concerns),
			Queue:       summary.Queue,
			RetryOn:     trimEntries(summary.RetryOn),
			DiscardOn:   trimEntries(summary.DiscardOn),
			Methods:     trimEntries(summary.Methods),
		}
		return printJSON(cmd, out)
	}

	fmt.Print(jobs.Format(summary, stdoutStyler()))
	return nil
}

func init() {
	rootCmd.AddCommand(jobsCmd)
}
