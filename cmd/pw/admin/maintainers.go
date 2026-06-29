// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package admin

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/getpatchwork/patchwork/cmd/pw/pw"
	"github.com/getpatchwork/patchwork/pkg/db"
)

type MaintainerCmd struct {
	List   MaintainerListCmd   `cmd:"" help:"List maintainers of a project."`
	Add    MaintainerAddCmd    `cmd:"" help:"Add a maintainer to a project."`
	Remove MaintainerRemoveCmd `cmd:"" help:"Remove a maintainer from a project."`
}

type MaintainerListCmd struct {
	Project string `arg:"" help:"Project linkname."`
}

func (c *MaintainerListCmd) Run(ctx *pw.Context) error {
	var project db.Project
	err := ctx.DB.NewSelect().Model(&project).
		Where("linkname = ?", c.Project).
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("project %q not found", c.Project)
	}

	var maintainers []db.ProjectMaintainer
	err = ctx.DB.NewSelect().Model(&maintainers).
		Relation("User").
		Where("project_id = ?", project.ID).
		Scan(ctx)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "USERNAME\tEMAIL\n")
	for _, m := range maintainers {
		if m.User != nil {
			fmt.Fprintf(w, "%s\t%s\n",
				m.User.Username,
				m.User.Email)
		}
	}
	return w.Flush()
}

type MaintainerAddCmd struct {
	Project  string `arg:"" help:"Project linkname."`
	Username string `arg:"" help:"Username to add as maintainer."`
}

func (c *MaintainerAddCmd) Run(ctx *pw.Context) error {
	var project db.Project
	err := ctx.DB.NewSelect().Model(&project).
		Where("linkname = ?", c.Project).
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("project %q not found", c.Project)
	}

	var user db.User
	err = ctx.DB.NewSelect().Model(&user).
		Where("username = ?", c.Username).
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("user %q not found", c.Username)
	}

	m := db.ProjectMaintainer{
		UserID:    user.ID,
		ProjectID: project.ID,
	}
	err = db.New(ctx, ctx.DB).Insert(&m)
	if err != nil {
		return err
	}

	fmt.Printf("Added %q as maintainer of %q\n", c.Username, c.Project)
	return nil
}

type MaintainerRemoveCmd struct {
	Project  string `arg:"" help:"Project linkname."`
	Username string `arg:"" help:"Username to remove."`
}

func (c *MaintainerRemoveCmd) Run(ctx *pw.Context) error {
	var project db.Project
	err := ctx.DB.NewSelect().Model(&project).
		Where("linkname = ?", c.Project).
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("project %q not found", c.Project)
	}

	var user db.User
	err = ctx.DB.NewSelect().Model(&user).
		Where("username = ?", c.Username).
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("user %q not found", c.Username)
	}

	res, err := ctx.DB.NewDelete().
		Model((*db.ProjectMaintainer)(nil)).
		Where("user_id = ?", user.ID).
		Where("project_id = ?", project.ID).
		Exec(ctx)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("%q is not a maintainer of %q", c.Username, c.Project)
	}

	fmt.Printf("Removed %q from maintainers of %q\n", c.Username, c.Project)
	return nil
}
