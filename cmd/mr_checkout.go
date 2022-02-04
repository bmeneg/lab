package cmd

import (
	"fmt"
	"os"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/rsteube/carapace"
	"github.com/spf13/cobra"
	gitlab "github.com/xanzy/go-gitlab"
	"github.com/zaquestion/lab/internal/action"
	"github.com/zaquestion/lab/internal/git"
	lab "github.com/zaquestion/lab/internal/gitlab"
)

// mrCheckoutConfig holds configuration values for calls to lab mr checkout
type mrCheckoutConfig struct {
	branch string
	remote string
	force  bool
	track  bool
}

var (
	mrCheckoutCfg mrCheckoutConfig
)

// listCmd represents the list command
var checkoutCmd = &cobra.Command{
	Use:   "checkout [remote] [<MR id or branch>]",
	Short: "Checkout an open merge request",
	Args:  cobra.RangeArgs(1, 2),
	Example: heredoc.Doc(`
		lab mr checkout origin 10
		lab mr checkout upstream -b a_branch_name
		lab mr checkout upstream -r a_remote_name
		lab mr checkout a_remote -f
		lab mr checkout upstream --https
		lab mr checkout upstream -t`),
	PersistentPreRun: labPersistentPreRun,
	Run: func(cmd *cobra.Command, args []string) {
		rn, mrID, err := parseArgsRemoteAndID(args)
		if err != nil {
			log.Fatal(err)
		}
		var targetRemote = defaultRemote
		if len(args) == 2 {
			// parseArgs above already validated this is a remote
			targetRemote = args[0]
		}

		mrs, err := lab.MRList(rn, gitlab.ListProjectMergeRequestsOptions{
			IIDs: []int{int(mrID)},
		}, 1)
		if err != nil {
			log.Fatal(err)
		}
		if len(mrs) < 1 {
			fmt.Printf("MR !%d not found\n", mrID)
			return
		}

		mr := mrs[0]
		// If the config does not specify a branch, use the mr source branch name
		if mrCheckoutCfg.branch == "" {
			mrCheckoutCfg.branch = mr.SourceBranch
		}

		// If track, make sure we have a remote for the mr author and then set
		// the fetchToRef to the mr author/sourceBranch
		trackRef := ""
		if mrCheckoutCfg.track {
			// Check if remote already exists
			project, err := lab.GetProject(mr.SourceProjectID)
			if err != nil {
				log.Fatal(err)
			}

			remotes, err := git.Remotes()
			if err != nil {
				log.Fatal(err)
			}

			remoteName := ""
			if mrCheckoutCfg.remote != "" {
				remoteName = mrCheckoutCfg.remote
			} else {
				for _, remote := range remotes {
					path, err := git.PathWithNamespace(remote)
					if err != nil {
						continue
					}
					if path == project.PathWithNamespace {
						remoteName = remote
					}
				}
			}

			if remoteName == "" {
				remoteName = mr.Author.Username
				urlToRepo := labURLToRepo(project)
				err := git.RemoteAdd(remoteName, urlToRepo, ".")
				if err != nil {
					log.Fatal(err)
				}
			}

			trackRef = fmt.Sprintf("%s/%s", remoteName, mr.SourceBranch)
		}

		fmt.Println("branch name:", mrCheckoutCfg.branch)
		err = git.New("show-ref", "--verify", "--quiet", "refs/heads/"+mrCheckoutCfg.branch).Run()
		if err == nil {
			fmt.Println("entrou")
			if mrCheckoutCfg.force {
				if err := git.New("branch", "-D", mrCheckoutCfg.branch).Run(); err != nil {
					log.Fatal(err)
				}
			} else {
				fmt.Println("ERROR: mr", mrID, "branch", mrCheckoutCfg.branch, "already exists.")
				os.Exit(1)
			}
		}
		fmt.Println("passou")

		// https://docs.gitlab.com/ce/user/project/merge_requests/#checkout-merge-requests-locally
		mrRef := fmt.Sprintf("refs/merge-requests/%d/head", mrID)
		fetchRefSpec := fmt.Sprintf("%s:%s", mrRef, mrCheckoutCfg.branch)
		if err := git.New("fetch", targetRemote, fetchRefSpec).Run(); err != nil {
			log.Fatal(err)
		}

		// Check out branch
		if err := git.New("checkout", mrCheckoutCfg.branch).Run(); err != nil {
			log.Fatal(err)
		}

		if mrCheckoutCfg.track {
			if err := git.New("branch", "-u", trackRef).Run(); err != nil {
				log.Fatal(err)
			}
		}
	},
}

func init() {
	checkoutCmd.Flags().StringVarP(&mrCheckoutCfg.branch, "branch", "b", "", "checkout merge request with <branch> name")
	checkoutCmd.Flags().StringVarP(&mrCheckoutCfg.remote, "remote", "r", "", "if tracking, force <remote> name")
	checkoutCmd.Flags().BoolVarP(&mrCheckoutCfg.track, "track", "t", false, "set to track remote branch, adds remote if needed")
	// useHTTP is defined in "project_create.go"
	checkoutCmd.Flags().BoolVar(&useHTTP, "http", false, "checkout using HTTP protocol instead of SSH")
	checkoutCmd.Flags().BoolVarP(&mrCheckoutCfg.force, "force", "f", false, "force branch checkout and override existing branch")
	mrCmd.AddCommand(checkoutCmd)
	carapace.Gen(checkoutCmd).PositionalCompletion(
		carapace.ActionCallback(func(c carapace.Context) carapace.Action {
			c.Args = []string{"origin"}
			return action.MergeRequests(mrList).Invoke(c).ToA()
		}),
	)
}
