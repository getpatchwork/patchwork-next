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

type TagCmd struct {
	List   TagListCmd   `cmd:"" help:"List tags."`
	Create TagCreateCmd `cmd:"" help:"Create a tag."`
	Delete TagDeleteCmd `cmd:"" help:"Delete a tag."`
}

type TagListCmd struct{}

func (c *TagListCmd) Run(ctx *pw.Context) error {
	var tags []db.Tag
	err := ctx.DB.NewSelect().Model(&tags).
		OrderExpr("id ASC").
		Scan(ctx)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "ID\tNAME\tABBREV\tPATTERN\tSHOW COLUMN\n")
	for _, t := range tags {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%v\n",
			t.ID, t.Name, t.Abbrev, t.Pattern, t.ShowColumn)
	}
	return w.Flush()
}

type TagCreateCmd struct {
	Name       string `required:"" short:"n" help:"Tag name."`
	Pattern    string `required:"" short:"p" help:"Regex pattern to match."`
	Abbrev     string `required:"" short:"a" help:"Short abbreviation."`
	ShowColumn bool   `name:"show-column" default:"true" help:"Show in list columns."`
}

func (c *TagCreateCmd) Run(ctx *pw.Context) error {
	tag := db.Tag{
		Name:       c.Name,
		Pattern:    c.Pattern,
		Abbrev:     c.Abbrev,
		ShowColumn: c.ShowColumn,
	}
	err := db.New(ctx, ctx.DB).Insert(&tag)
	if err != nil {
		return err
	}

	fmt.Printf("Created tag %q (id=%d)\n", tag.Name, tag.ID)
	return nil
}

type TagDeleteCmd struct {
	Force bool   `short:"f" help:"Skip confirmation."`
	Name  string `arg:"" help:"Tag name to delete."`
}

func (c *TagDeleteCmd) Run(ctx *pw.Context) error {
	var tag db.Tag
	err := ctx.DB.NewSelect().Model(&tag).
		Where("name = ?", c.Name).
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("tag %q not found", c.Name)
	}

	if !c.Force {
		fmt.Printf("Delete tag %q (id=%d)? [y/N] ", tag.Name, tag.ID)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	_, err = ctx.DB.NewDelete().Model((*db.Tag)(nil)).
		Where("id = ?", tag.ID).
		Exec(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("Deleted tag %q\n", c.Name)
	return nil
}
