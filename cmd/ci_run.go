package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/pkg/errors"
	"github.com/rsteube/carapace"
	"github.com/spf13/cobra"
	gitlab "github.com/xanzy/go-gitlab"
	"github.com/zaquestion/lab/internal/action"
	lab "github.com/zaquestion/lab/internal/gitlab"
)

// ciCreateCmd represents the run command
var ciCreateCmd = &cobra.Command{
	Use:     "create [branch]",
	Aliases: []string{"run"},
	Short:   "Create a CI pipeline",
	Long: heredoc.Doc(`
		Run the CI pipeline for the given or current branch if none provided.
		This API uses your GitLab token to create CI pipelines

		Project will be inferred from branch if not provided

		Note: "lab ci create" differs from "lab ci trigger" which is a
		different API`),
	Example: heredoc.Doc(`
		lab ci create feature_branch
		lab ci create -p engineering/integration_tests master`),
	PersistentPreRun: labPersistentPreRun,
	Run: func(cmd *cobra.Command, args []string) {
		forMR, err := cmd.Flags().GetBool("merge-request")
		if err != nil {
			log.Fatal(err)
		}

		project, err := cmd.Flags().GetString("project")
		if err != nil {
			log.Fatal(err)
		}

		if forMR {
			if project != "" {
				log.Fatal("option --merge-request cannot be combined with --project/-p")
			}

			rn, mrNum, err := parseArgsWithGitBranchMR(args)
			if err != nil {
				log.Fatal(err)
			}

			pipeline, err := lab.CIMRCreate(rn, int(mrNum))
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("%s\n", pipeline.WebURL)
		} else {
			var pid interface{}

			rn, branch, err := parseArgsRemoteAndBranch(args)
			if err != nil {
				log.Fatal(err)
			}
			pid = rn

			if project != "" {
				p, err := lab.FindProject(project)
				if err != nil {
					log.Fatal(err)
				}
				pid = p.ID
			}

			pipeline, err := lab.CICreate(pid, &gitlab.CreatePipelineOptions{Ref: &branch})
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("%s\n", pipeline.WebURL)
		}
	},
}

var ciTriggerCmd = &cobra.Command{
	Use:   "trigger [branch]",
	Short: "Trigger a CI pipeline",
	Long: heredoc.Doc(`
		Runs a trigger for a CI pipeline on the given or current branch if none provided.
		This API supports variables and must be called with a trigger token or from within GitLab CI.

		Project will be inferred from branch if not provided

		Note: "lab ci trigger" differs from "lab ci create" which is a different API`),
	Example: heredoc.Doc(`
		lab ci trigger feature_branch
		lab ci trigger -p engineering/integration_tests master
		lab ci trigger -p engineering/integration_tests -v foo=bar master`),
	PersistentPreRun: labPersistentPreRun,
	Run: func(cmd *cobra.Command, args []string) {
		var pid interface{}

		project, err := cmd.Flags().GetString("project")
		if err != nil {
			log.Fatal(err)
		}

		rn, branch, err := parseArgsRemoteAndBranch(args)
		if err != nil {
			log.Fatal(err)
		}
		pid = rn

		if project != "" {
			p, err := lab.FindProject(project)
			if err != nil {
				log.Fatal(err)
			}
			pid = p.ID
		}

		token, err := cmd.Flags().GetString("token")
		if err != nil {
			log.Fatal(err)
		}
		vars, err := cmd.Flags().GetStringSlice("variable")
		if err != nil {
			log.Fatal(err)
		}
		ciVars, err := parseCIVariables(vars)
		if err != nil {
			log.Fatal(err)
		}
		pipeline, err := lab.CITrigger(pid, gitlab.RunPipelineTriggerOptions{
			Ref:       &branch,
			Token:     &token,
			Variables: ciVars,
		})
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("%s\n", pipeline.WebURL)
	},
}

func parseCIVariables(vars []string) (map[string]string, error) {
	variables := make(map[string]string)
	for _, v := range vars {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) < 2 {
			return nil, errors.Errorf("Invalid Variable: \"%s\", Variables must be in the format key=value", v)
		}
		variables[parts[0]] = parts[1]

	}
	return variables, nil
}

func init() {
	ciCreateCmd.Flags().StringP("project", "p", "", "project to create pipeline on")
	ciCreateCmd.Flags().Bool("merge-request", false, "use merge request pipeline if enabled")
	ciCmd.AddCommand(ciCreateCmd)
	carapace.Gen(ciCreateCmd).PositionalCompletion(
		action.Remotes(),
	)

	ciTriggerCmd.Flags().StringP("project", "p", "", "project to run pipeline trigger on")
	ciTriggerCmd.Flags().StringP("token", "t", os.Getenv("CI_JOB_TOKEN"), "pipeline trigger token, optional if run within GitLabCI")
	ciTriggerCmd.Flags().StringSliceP("variable", "v", []string{}, "variables to pass to pipeline")

	ciCmd.AddCommand(ciTriggerCmd)
	carapace.Gen(ciTriggerCmd).PositionalCompletion(
		action.RemoteBranches(-1),
	)
}
