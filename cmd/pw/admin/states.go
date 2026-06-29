// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package admin

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/getpatchwork/patchwork/cmd/pw/pw"
	"github.com/getpatchwork/patchwork/pkg/db"
)

type StateCmd struct {
	List   StateListCmd   `cmd:"" help:"List states."`
	Create StateCreateCmd `cmd:"" help:"Create a state."`
	Delete StateDeleteCmd `cmd:"" help:"Delete a state."`
}

type StateListCmd struct{}

func (c *StateListCmd) Run(ctx *pw.Context) error {
	var states []db.State
	err := ctx.DB.NewSelect().Model(&states).
		OrderExpr("ordering ASC").
		Scan(ctx)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "ID\tNAME\tSLUG\tORDERING\tACTION REQUIRED\n")
	for _, s := range states {
		fmt.Fprintf(w, "%d\t%s\t%s\t%d\t%v\n",
			s.ID, s.Name, s.Slug, s.Ordering, s.ActionRequired)
	}
	return w.Flush()
}

type StateCreateCmd struct {
	Name           string `required:"" short:"n" help:"State name."`
	Slug           string `required:"" short:"s" help:"URL-safe slug."`
	Ordering       int    `required:"" short:"o" help:"Sort order."`
	ActionRequired bool   `name:"action-required" help:"Whether this state requires action."`
}

func (c *StateCreateCmd) Run(ctx *pw.Context) error {
	state := db.State{
		Name:           c.Name,
		Slug:           c.Slug,
		Ordering:       c.Ordering,
		ActionRequired: c.ActionRequired,
	}
	err := db.New(ctx, ctx.DB).Insert(&state)
	if err != nil {
		return err
	}

	fmt.Printf("Created state %q (id=%d)\n", state.Name, state.ID)
	return nil
}

type StateDeleteCmd struct {
	Force bool   `short:"f" help:"Skip confirmation."`
	Slug  string `arg:"" help:"State slug to delete."`
}

func (c *StateDeleteCmd) Run(ctx *pw.Context) error {
	var state db.State
	err := ctx.DB.NewSelect().Model(&state).
		Where("slug = ?", c.Slug).
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("state %q not found", c.Slug)
	}

	if !c.Force {
		fmt.Printf("Delete state %q (id=%d)? [y/N] ", state.Name, state.ID)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	_, err = ctx.DB.NewDelete().Model((*db.State)(nil)).
		Where("id = ?", state.ID).
		Exec(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("Deleted state %q\n", state.Name)
	return nil
}
