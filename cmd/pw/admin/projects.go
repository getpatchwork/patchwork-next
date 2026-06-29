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

type ProjectCmd struct {
	List   ProjectListCmd   `cmd:"" help:"List all projects."`
	Show   ProjectShowCmd   `cmd:"" help:"Show project details."`
	Create ProjectCreateCmd `cmd:"" help:"Create a new project."`
	Update ProjectUpdateCmd `cmd:"" help:"Update a project."`
	Delete ProjectDeleteCmd `cmd:"" help:"Delete a project."`
}

type ProjectListCmd struct{}

func (c *ProjectListCmd) Run(ctx *pw.Context) error {
	var projects []db.Project
	err := ctx.DB.NewSelect().Model(&projects).
		OrderExpr("id ASC").
		Scan(ctx)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "ID\tLINKNAME\tNAME\tLIST EMAIL\n")
	for _, p := range projects {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\n",
			p.ID, p.Linkname, p.Name, p.Listemail)
	}
	return w.Flush()
}

type ProjectShowCmd struct {
	Linkname string `arg:"" help:"Project linkname."`
}

func (c *ProjectShowCmd) Run(ctx *pw.Context) error {
	var project db.Project
	err := ctx.DB.NewSelect().Model(&project).
		Where("linkname = ?", c.Linkname).
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("project %q not found", c.Linkname)
	}

	printField("ID", fmt.Sprintf("%d", project.ID))
	printField("Linkname", project.Linkname)
	printField("Name", project.Name)
	printField("List ID", project.Listid)
	printField("List email", project.Listemail)
	printField("Web URL", project.WebURL)
	printField("SCM URL", project.ScmURL)
	printField("Web SCM URL", project.WebScmURL)
	printField("List archive URL", project.ListArchiveURL)
	printField("Subject match", project.SubjectMatch)
	printField("Commit URL format", project.CommitURLFormat)

	return nil
}

type ProjectCreateCmd struct {
	Name           string `required:"" short:"n" help:"Display name."`
	Linkname       string `required:"" short:"l" help:"URL-safe identifier."`
	ListID         string `required:"" short:"i" help:"Mailing list ID."`
	ListEmail      string `required:"" short:"e" help:"Mailing list email."`
	WebURL         string `name:"web-url" help:"Project website URL."`
	ScmURL         string `name:"scm-url" help:"Source code management URL."`
	WebScmURL      string `name:"webscm-url" help:"Web SCM URL."`
	ListArchiveURL string `name:"list-archive-url" help:"List archive URL."`
	SubjectMatch   string `name:"subject-match" help:"Subject filter regex."`
	CommitURL      string `name:"commit-url-format" help:"Commit URL format string."`
}

func (c *ProjectCreateCmd) Run(ctx *pw.Context) error {
	project := db.Project{
		Name:                 c.Name,
		Linkname:             c.Linkname,
		Listid:               c.ListID,
		Listemail:            c.ListEmail,
		WebURL:               c.WebURL,
		ScmURL:               c.ScmURL,
		WebScmURL:            c.WebScmURL,
		ListArchiveURL:       c.ListArchiveURL,
		SubjectMatch:         c.SubjectMatch,
		CommitURLFormat:      c.CommitURL,
		ListArchiveURLFormat: "",
	}
	err := db.New(ctx, ctx.DB).Insert(&project)
	if err != nil {
		return err
	}

	fmt.Printf("Created project %q (id=%d)\n", project.Linkname, project.ID)
	return nil
}

type ProjectUpdateCmd struct {
	Linkname       string `arg:"" help:"Project linkname."`
	Name           string `short:"n" help:"Display name."`
	ListID         string `name:"list-id" help:"Mailing list ID."`
	ListEmail      string `name:"list-email" help:"Mailing list email."`
	WebURL         string `name:"web-url" help:"Project website URL."`
	ScmURL         string `name:"scm-url" help:"Source code management URL."`
	WebScmURL      string `name:"webscm-url" help:"Web SCM URL."`
	ListArchiveURL string `name:"list-archive-url" help:"List archive URL."`
	SubjectMatch   string `name:"subject-match" help:"Subject filter regex."`
	CommitURL      string `name:"commit-url-format" help:"Commit URL format string."`
}

func (c *ProjectUpdateCmd) Run(ctx *pw.Context) error {
	var project db.Project
	err := ctx.DB.NewSelect().Model(&project).
		Where("linkname = ?", c.Linkname).
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("project %q not found", c.Linkname)
	}

	q := ctx.DB.NewUpdate().Model(&project).Where("id = ?", project.ID)
	updated := false
	if c.Name != "" {
		q = q.Set("name = ?", c.Name)
		updated = true
	}
	if c.ListID != "" {
		q = q.Set("listid = ?", c.ListID)
		updated = true
	}
	if c.ListEmail != "" {
		q = q.Set("listemail = ?", c.ListEmail)
		updated = true
	}
	if c.WebURL != "" {
		q = q.Set("web_url = ?", c.WebURL)
		updated = true
	}
	if c.ScmURL != "" {
		q = q.Set("scm_url = ?", c.ScmURL)
		updated = true
	}
	if c.WebScmURL != "" {
		q = q.Set("webscm_url = ?", c.WebScmURL)
		updated = true
	}
	if c.ListArchiveURL != "" {
		q = q.Set("list_archive_url = ?", c.ListArchiveURL)
		updated = true
	}
	if c.SubjectMatch != "" {
		q = q.Set("subject_match = ?", c.SubjectMatch)
		updated = true
	}
	if c.CommitURL != "" {
		q = q.Set("commit_url_format = ?", c.CommitURL)
		updated = true
	}

	if !updated {
		return fmt.Errorf("no fields to update")
	}

	_, err = q.Exec(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("Updated project %q\n", c.Linkname)
	return nil
}

type ProjectDeleteCmd struct {
	Force    bool   `short:"f" help:"Skip confirmation."`
	Linkname string `arg:"" help:"Project linkname."`
}

func (c *ProjectDeleteCmd) Run(ctx *pw.Context) error {
	var project db.Project
	err := ctx.DB.NewSelect().Model(&project).
		Where("linkname = ?", c.Linkname).
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("project %q not found", c.Linkname)
	}

	if !c.Force {
		fmt.Printf("Delete project %q (id=%d)? [y/N] ", project.Linkname, project.ID)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	_, err = ctx.DB.NewDelete().Model((*db.Project)(nil)).
		Where("id = ?", project.ID).
		Exec(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("Deleted project %q\n", project.Linkname)
	return nil
}

func printField(label, value string) {
	if value != "" {
		fmt.Printf("%-20s %s\n", label+":", value)
	}
}
