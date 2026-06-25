package main

import (
	"fmt"
	"strconv"
	"strings"

	internalgit "github.com/amo/devlog/internal/git"
	sessionstore "github.com/amo/devlog/internal/store"
	"github.com/amo/devlog/internal/todo"
	"github.com/spf13/cobra"
)

func newTodoCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "todo",
		Short:        "Manage repo-wide todos.",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newTodoListCommand())
	cmd.AddCommand(newTodoAddCommand())
	cmd.AddCommand(newTodoEditCommand())
	cmd.AddCommand(newTodoDoneCommand())
	cmd.AddCommand(newTodoReopenCommand())
	cmd.AddCommand(newTodoDeleteCommand())

	return cmd
}

func newTodoListCommand() *cobra.Command {
	var filters todoFilterFlags
	var showIDs bool

	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List repo-wide todos.",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if filters.all && filters.done {
				return fmt.Errorf("todo list: --all and --done cannot be used together")
			}

			store, err := newTodoStore()
			if err != nil {
				return err
			}

			items, err := store.List(filters.filter())
			if err != nil {
				return err
			}

			_, err = fmt.Fprint(cmd.OutOrStdout(), renderTodoList(items, showIDs))
			return err
		},
	}

	filters.addFlags(cmd)
	cmd.Flags().BoolVar(&showIDs, "ids", false, "Show internal todo IDs")

	return cmd
}

func newTodoAddCommand() *cobra.Command {
	var message string

	cmd := &cobra.Command{
		Use:          "add [text]",
		Short:        "Add a repo-wide todo.",
		Args:         cobra.ArbitraryArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			text, err := resolveTodoText(message, args, "todo")
			if err != nil {
				return err
			}

			root, err := internalgit.RepoRoot()
			if err != nil {
				return err
			}

			store, err := todo.NewStore(root)
			if err != nil {
				return err
			}

			sessionID, branch, err := activeTodoAttribution(root)
			if err != nil {
				return err
			}

			item, err := store.Add(todo.AddInput{Text: text, SessionID: sessionID, Branch: branch})
			if err != nil {
				return err
			}

			_, err = fmt.Fprint(cmd.OutOrStdout(), renderTodoAddConfirmation(item))
			return err
		},
	}

	cmd.Flags().StringVarP(&message, "message", "m", "", "Todo text")

	return cmd
}

func newTodoEditCommand() *cobra.Command {
	var filters todoFilterFlags
	var message string

	cmd := &cobra.Command{
		Use:          "edit <ref> [text]",
		Short:        "Edit an open todo.",
		Args:         cobra.ArbitraryArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, err := todoRefFromArgs(args)
			if err != nil {
				return err
			}

			text, err := resolveTodoText(message, args[1:], "todo")
			if err != nil {
				return err
			}

			store, err := newTodoStore()
			if err != nil {
				return err
			}

			id, err := resolveTodoRef(store, ref, filters.filter())
			if err != nil {
				return err
			}

			if err := store.UpdateText(id, text); err != nil {
				return err
			}

			item, err := findTodoByID(store, id)
			if err != nil {
				return err
			}

			_, err = fmt.Fprint(cmd.OutOrStdout(), renderTodoConfirmation("Edited todo", item))
			return err
		},
	}

	filters.addFlags(cmd)
	cmd.Flags().StringVarP(&message, "message", "m", "", "Updated todo text")

	return cmd
}

func newTodoDoneCommand() *cobra.Command {
	return newTodoTransitionCommand("done", "Mark a todo done.", "Completed todo", func(store *todo.Store, id string) error {
		return store.Complete(id)
	})
}

func newTodoReopenCommand() *cobra.Command {
	return newTodoTransitionCommand("reopen", "Reopen a done todo.", "Reopened todo", func(store *todo.Store, id string) error {
		return store.Reopen(id)
	})
}

func newTodoDeleteCommand() *cobra.Command {
	return newTodoTransitionCommand("delete", "Delete a todo.", "Deleted todo", func(store *todo.Store, id string) error {
		return store.Delete(id)
	})
}

func newTodoTransitionCommand(use, short, confirmation string, transition func(*todo.Store, string) error) *cobra.Command {
	var filters todoFilterFlags

	cmd := &cobra.Command{
		Use:          use + " <ref>",
		Short:        short,
		Args:         cobra.ArbitraryArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, err := todoRefFromArgs(args)
			if err != nil {
				return err
			}

			store, err := newTodoStore()
			if err != nil {
				return err
			}

			id, err := resolveTodoRef(store, ref, filters.filter())
			if err != nil {
				return err
			}

			if err := transition(store, id); err != nil {
				return err
			}

			item, err := findTodoByID(store, id)
			if err != nil {
				return err
			}

			_, err = fmt.Fprint(cmd.OutOrStdout(), renderTodoConfirmation(confirmation, item))
			return err
		},
	}

	filters.addFlags(cmd)

	return cmd
}

func newTodoStore() (*todo.Store, error) {
	root, err := internalgit.RepoRoot()
	if err != nil {
		return nil, err
	}
	return todo.NewStore(root)
}

type todoFilterFlags struct {
	all     bool
	done    bool
	branch  string
	session string
}

func (f *todoFilterFlags) addFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&f.all, "all", false, "Include open and done todos")
	cmd.Flags().BoolVar(&f.done, "done", false, "Include only done todos")
	cmd.Flags().StringVar(&f.branch, "branch", "", "Filter by branch")
	cmd.Flags().StringVar(&f.session, "session", "", "Filter by session ID")
}

func (f *todoFilterFlags) filter() todo.Filter {
	return todoListFilter(f.all, f.done, f.session, f.branch)
}

func todoListFilter(all, done bool, sessionID, branch string) todo.Filter {
	filter := todo.Filter{
		IncludeOpen:     true,
		SessionID:       strings.TrimSpace(sessionID),
		Branch:          strings.TrimSpace(branch),
		MatchSessionAny: strings.TrimSpace(sessionID) == "",
		MatchBranchAny:  strings.TrimSpace(branch) == "",
	}
	if all {
		filter.IncludeDone = true
	}
	if done {
		filter.IncludeOpen = false
		filter.IncludeDone = true
	}
	return filter
}

func activeTodoAttribution(root string) (string, string, error) {
	s, err := sessionstore.New(root)
	if err != nil {
		return "", "", err
	}

	records, err := s.ListSessions()
	if err != nil {
		return "", "", err
	}

	var active *sessionstore.SessionRecord
	for i := range records {
		if records[i].Closed {
			continue
		}
		if active != nil {
			return "", "", fmt.Errorf("more than one active session exists; check .devlog/sessions/ for open sessions")
		}
		active = &records[i]
	}
	if active == nil {
		return "", "", nil
	}

	return active.ID, active.Branch, nil
}

func resolveTodoText(flagMsg string, args []string, label string) (string, error) {
	text := strings.TrimSpace(flagMsg)
	if text == "" {
		text = strings.TrimSpace(strings.Join(args, " "))
	}
	if text == "" {
		return "", fmt.Errorf("%s text is required", label)
	}
	return text, nil
}

func todoRefFromArgs(args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("todo ref is required")
	}
	ref := strings.TrimSpace(args[0])
	if ref == "" {
		return "", fmt.Errorf("todo ref is required")
	}
	return ref, nil
}

func resolveTodoRef(store *todo.Store, ref string, filter todo.Filter) (string, error) {
	if n, err := strconv.Atoi(ref); err == nil && n >= 1 {
		items, listErr := store.List(filter)
		if listErr != nil {
			return "", listErr
		}
		if n > len(items) {
			return "", fmt.Errorf("todo number %d out of range (list has %d items)", n, len(items))
		}
		return items[n-1].ID, nil
	}

	items, err := store.Load()
	if err != nil {
		return "", err
	}
	for _, item := range items {
		if item.ID == ref {
			return ref, nil
		}
	}
	return "", fmt.Errorf("todo %q not found", ref)
}

func findTodoByID(store *todo.Store, id string) (todo.Item, error) {
	items, err := store.Load()
	if err != nil {
		return todo.Item{}, err
	}
	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}
	return todo.Item{}, fmt.Errorf("todo %q not found", id)
}

func todoCheckbox(item todo.Item) string {
	if item.Status == todo.StatusDone {
		return "[x]"
	}
	return "[ ]"
}

func oneLineTodoText(text string) string {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return "(empty)"
	}
	return strings.Join(fields, " ")
}

func renderTodoList(items []todo.Item, showIDs bool) string {
	var b strings.Builder
	b.WriteString(cliTitleStyle.Render("Todos"))
	b.WriteByte('\n')

	if len(items) == 0 {
		b.WriteString("  None\n")
		return b.String()
	}

	for i, item := range items {
		fmt.Fprintf(&b, "  %d. %s ", i+1, todoCheckbox(item))
		b.WriteString(cliValueStyle.Render(oneLineTodoText(item.Text)))
		b.WriteByte('\n')

		if showIDs {
			writeCLIField(&b, "id", item.ID)
		}
	}

	return b.String()
}

func renderTodoConfirmation(title string, item todo.Item) string {
	var b strings.Builder
	b.WriteString(cliTitleStyle.Render(title))
	b.WriteByte('\n')
	fmt.Fprintf(&b, "  %s ", todoCheckbox(item))
	b.WriteString(cliValueStyle.Render(oneLineTodoText(item.Text)))
	b.WriteByte('\n')
	return b.String()
}

func renderTodoAddConfirmation(item todo.Item) string {
	var b strings.Builder
	b.WriteString(cliTitleStyle.Render("Added todo"))
	b.WriteByte('\n')
	fmt.Fprintf(&b, "  %s ", todoCheckbox(item))
	b.WriteString(cliValueStyle.Render(oneLineTodoText(item.Text)))
	b.WriteByte('\n')

	if item.SessionID != "" {
		writeCLIField(&b, "session", item.SessionID)
	}
	if item.Branch != "" {
		writeCLIField(&b, "branch", item.Branch)
	}

	return b.String()
}
