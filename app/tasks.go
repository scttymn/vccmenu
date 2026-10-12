package app

import (
	"context"
	"errors"
	"fmt"
	"github.com/scttymn/gantry/token"
	"io"
	"log/slog"
	"time"

	"github.com/scttymn/gantry/db"
)

// Tasks are the app's own commands, Rails' rake tasks: `vccmenu task NAME
// [ARGS...]` runs one, and `gantry task NAME` runs it in development. Add
// one here:
//
//	{Name: "backfill-slugs", Help: "Give every post a slug", Run: backfillSlugs},
var Tasks = []Task{{Name: "invite-editor", Help: "Make a one-use editor setup code, valid for 24 hours", Run: func(ctx context.Context, env TaskEnv, args []string) error {
	if len(args) != 0 {
		return errors.New("invite-editor takes no arguments")
	}
	key, digest := token.New("")
	if _, err := env.DB.Write.ExecContext(ctx, "INSERT INTO invitations (token_digest, expires_at) VALUES (?, ?)", digest, time.Now().Add(24*time.Hour).Unix()); err != nil {
		return err
	}
	fmt.Fprintln(env.Out, "Open /setup on the menu app and enter this one-use code (valid for 24 hours):")
	fmt.Fprintln(env.Out, key)
	return nil
}}}

// Task is one of the app's commands.
type Task struct {
	Name string
	Help string // one line, for `vccmenu tasks`
	Run  func(ctx context.Context, env TaskEnv, args []string) error
}

// TaskEnv is what a task works with: the migrated database, a logger, and
// where to print.
type TaskEnv struct {
	DB  *db.DB
	Log *slog.Logger
	Out io.Writer
}
