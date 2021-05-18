package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	gitlab "github.com/xanzy/go-gitlab"
	lab "github.com/zaquestion/lab/internal/gitlab"
)

var (
	todoType   string
	todoNumRet string
	targetType string
	todoPretty bool
)

var todoListCmd = &cobra.Command{
	Use:              "list",
	Aliases:          []string{"ls"},
	Short:            "List todos",
	Long:             ``,
	Example:          `lab todo list                        # list open todos"`,
	PersistentPreRun: LabPersistentPreRun,
	Run: func(cmd *cobra.Command, args []string) {
		todos, err := todoList(args)
		if err != nil {
			log.Fatal(err)
		}

		pager := NewPager(cmd.Flags())
		defer pager.Close()

		red := color.New(color.FgRed).SprintFunc()
		green := color.New(color.FgGreen).SprintFunc()
		cyan := color.New(color.FgCyan).SprintFunc()

		for _, todo := range todos {
			if !todoPretty || todo.TargetType == "DesignManagement::Design" || todo.TargetType == "AlertManagement::Alert" {
				fmt.Printf("%d %s\n", todo.ID, todo.TargetURL)
				continue
			}

			delim := "merge_requests/"
			if todo.TargetType == "Issue" {
				delim = "issues/"
			}

			s := strings.Split(todo.TargetURL, delim)
			s = strings.Split(s[1], "#")
			id, err := strconv.Atoi(s[0])
			if err != nil {
				log.Fatal(err)
			}

			var (
				state string
				title string
			)
			if todo.TargetType == "MergeRequest" {
				mr, err := lab.MRGet(todo.Project.ID, id)
				if err != nil {
					log.Fatal(err)
				}
				state = mr.State
				if mr.State == "opened" && mr.WorkInProgress {
					state = "draft"
				}
				title = mr.Title
			} else {
				issue, err := lab.IssueGet(todo.Project.ID, id)
				if err != nil {
					log.Fatal(err)
				}
				state = issue.State
				title = issue.Title
			}

			switch state {
			case "opened":
				state = green("open  ")
			case "merged":
				state = cyan("merged")
			case "draft":
				state = green("draft ")
			default:
				state = red(state)
			}

			fmt.Printf("%s %d \"%s\" ", state, todo.ID, title)

			name := todo.Author.Name
			if lab.User() == todo.Author.Username {
				name = "you"
			}
			switch todo.ActionName {
			case "approval_required":
				fmt.Printf("(approval requested by %s)\n", name)
			case "assigned":
				fmt.Printf("(assigned to you by %s)\n", name)
			case "build_failed":
				fmt.Printf("(build failed)\n")
			case "directly_addressed":
				fmt.Printf("(%s directly addressed you)\n", name)
			case "marked":
				fmt.Printf("(Todo Entry added by you)\n")
			case "mentioned":
				fmt.Printf("(%s mentioned you)\n", name)
			case "merge_train_removed":
				fmt.Printf("(Merge Train was removed)\n")
			case "review_requested":
				fmt.Printf("(review requested by %s)\n", name)
			case "unmergeable":
				fmt.Printf("(Cannot be merged)\n")
			default:
				fmt.Printf("Unknown action %s\n", todo.ActionName)
			}

			fmt.Printf("       %s\n", todo.TargetURL)
		}
	},
}

func todoList(args []string) ([]*gitlab.Todo, error) {
	num, err := strconv.Atoi(todoNumRet)
	if err != nil {
		num = -1
	}

	opts := gitlab.ListTodosOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: num,
		},
	}

	var lstr = strings.ToLower(todoType)
	switch {
	case lstr == "mr":
		targetType = "MergeRequest"
		opts.Type = &targetType
	case lstr == "issue":
		targetType = "Issue"
		opts.Type = &targetType
	}

	return lab.TodoList(opts, num)
}

func init() {
	todoListCmd.Flags().BoolVarP(&todoPretty, "pretty", "p", false, "provide more infomation in output")
	todoListCmd.Flags().StringVarP(
		&todoType, "type", "t", "all",
		"filter todos by type (all/mr/issue)")
	todoListCmd.Flags().StringVarP(
		&todoNumRet, "number", "n", "10",
		"number of todos to return")

	todoCmd.AddCommand(todoListCmd)
}
