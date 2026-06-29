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

	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/cmd/pw/pw"
	"github.com/getpatchwork/patchwork/pkg/db"
)

type DelegateRuleCmd struct {
	List   DelegateRuleListCmd   `cmd:"" help:"List delegation rules for a project."`
	Create DelegateRuleCreateCmd `cmd:"" help:"Create a delegation rule."`
	Delete DelegateRuleDeleteCmd `cmd:"" help:"Delete a delegation rule."`
}

type DelegateRuleListCmd struct {
	Project string `arg:"" help:"Project linkname."`
}

func (c *DelegateRuleListCmd) Run(ctx *pw.Context) error {
	var project db.Project
	err := ctx.DB.NewSelect().Model(&project).
		Where("linkname = ?", c.Project).
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("project %q not found", c.Project)
	}

	var rules []db.DelegationRule
	err = ctx.DB.NewSelect().Model(&rules).
		Where("project_id = ?", project.ID).
		OrderExpr("priority ASC").
		Scan(ctx)
	if err != nil {
		return err
	}

	// resolve usernames
	userIDs := make([]int, 0, len(rules))
	for _, r := range rules {
		userIDs = append(userIDs, r.UserID)
	}
	var users []db.User
	if len(userIDs) > 0 {
		err = ctx.DB.NewSelect().Model(&users).
			Where("id IN (?)", bun.List(userIDs)).
			Scan(ctx)
		if err != nil {
			return err
		}
	}
	userMap := make(map[int]string, len(users))
	for _, u := range users {
		userMap[u.ID] = u.Username
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "ID\tPATH\tPRIORITY\tDELEGATE\n")
	for _, r := range rules {
		fmt.Fprintf(w, "%d\t%s\t%d\t%s\n",
			r.ID, r.Path, r.Priority, userMap[r.UserID])
	}
	return w.Flush()
}

type DelegateRuleCreateCmd struct {
	Project  string `required:"" name:"project" help:"Project linkname."`
	User     string `required:"" name:"user" help:"Delegate username."`
	Path     string `required:"" name:"path" help:"File path pattern."`
	Priority int    `name:"priority" default:"0" help:"Rule priority (lower = higher priority)."`
}

func (c *DelegateRuleCreateCmd) Run(ctx *pw.Context) error {
	var project db.Project
	err := ctx.DB.NewSelect().Model(&project).
		Where("linkname = ?", c.Project).
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("project %q not found", c.Project)
	}

	var user db.User
	err = ctx.DB.NewSelect().Model(&user).
		Where("username = ?", c.User).
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("user %q not found", c.User)
	}

	rule := db.DelegationRule{
		ProjectID: project.ID,
		UserID:    user.ID,
		Path:      c.Path,
		Priority:  c.Priority,
	}
	err = db.New(ctx, ctx.DB).Insert(&rule)
	if err != nil {
		return err
	}

	fmt.Printf("Created delegation rule (id=%d) %s -> %s\n",
		rule.ID, c.Path, c.User)
	return nil
}

type DelegateRuleDeleteCmd struct {
	Force bool `short:"f" help:"Skip confirmation."`
	ID    int  `arg:"" help:"Rule ID to delete."`
}

func (c *DelegateRuleDeleteCmd) Run(ctx *pw.Context) error {
	var rule db.DelegationRule
	err := ctx.DB.NewSelect().Model(&rule).
		Where("id = ?", c.ID).
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("delegation rule %d not found", c.ID)
	}

	if !c.Force {
		fmt.Printf("Delete delegation rule %d (path=%q)? [y/N] ",
			rule.ID, rule.Path)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	_, err = ctx.DB.NewDelete().Model((*db.DelegationRule)(nil)).
		Where("id = ?", rule.ID).
		Exec(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("Deleted delegation rule %d\n", c.ID)
	return nil
}
