package seeds

import (
	"context"
	"github.com/scttymn/gantry/db"
	"github.com/scttymn/vccmenu/app/menu"
)

func Run(ctx context.Context, d *db.DB) error { return menu.Seed(ctx, d) }
