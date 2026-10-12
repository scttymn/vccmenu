// Package app is the app: every route, and the controllers behind them,
// built from what they depend on.
package app

import (
	"github.com/scttymn/gantry/auth"
	"log/slog"
	"net/http"
	"time"

	"github.com/scttymn/gantry/db"
	"github.com/scttymn/gantry/jobs"
	"github.com/scttymn/gantry/live"
	"github.com/scttymn/gantry/sign"
	"github.com/scttymn/gantry/web"

	"github.com/scttymn/vccmenu/app/menu"
	"github.com/scttymn/vccmenu/assets"
)

// App is what the controllers share.
type App struct {
	DB     *db.DB
	Log    *slog.Logger
	Signer sign.Signer
	Jobs   *jobs.Queue // the background jobs, defined in app/jobs.go
	// Live sends pages their changes as they happen: a page draws
	// @a.Live.Source("post:" + id) to listen, and a controller or job calls
	// a.Live.Broadcast(ctx, "post:"+id, turbo.Replace(...)), or
	// a.Live.Refresh("post:"+id, turbo.RequestID(r)).
	Live *live.Hub
}

// Handler is every route, in gantry's middleware.
func (a *App) Handler() http.Handler {
	rt := web.NewRouter(a.Log, nil)
	// Errors are assets/public's pages (404.html, 500.html ...), plain
	// files, so they show even when the app can't draw a page.
	rt.Public = assets.All

	login := &auth.Auth{DB: a.DB, Signer: a.Signer, Log: a.Log, Limits: &web.Limits{}, Idle: 30 * 24 * time.Hour, Lifetime: 90 * 24 * time.Hour, Paths: auth.Paths{AfterLogin: "/admin"}}
	c := menu.Controller{DB: a.DB, Auth: login}
	login.Views.Login = c.Login
	login.Routes(rt)
	rt.Handle("GET /{$}", c.Display)
	rt.Handle("GET /api/menu", c.Public)
	rt.Handle("GET /setup", c.Setup)
	rt.Handle("POST /setup", c.CreateEditor)
	rt.Scope("/admin", web.Pipeline{login.Required}, func(s *web.Scope) {
		s.Handle("GET /{$}", c.Editor)
		s.Handle("GET /menu", c.Read)
		s.Handle("PUT /menu", c.Save)
		s.Handle("GET /password", c.Password)
		s.Handle("POST /password", c.ChangePassword)
		s.Handle("GET /users", c.Users)
		s.Handle("POST /users", c.AddUser)
	})

	// The pages' live streams: put it behind the filters that decide who
	// may listen, as the pages it serves are.
	rt.Handle("GET /live", a.Live.Serve)

	assets.Routes(rt.Mount)
	return rt.Handler()
}
