// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/getpatchwork/patchwork/cmd/pw/pw"
	"github.com/getpatchwork/patchwork/pkg/db"
	"github.com/getpatchwork/patchwork/pkg/events"
)

type GenerateCmd struct {
	Name    string `required:"" help:"Name of the project."`
	Series  int    `required:"" help:"Number of series to generate. Each series will have up to 20 patches."`
	Authors int    `required:"" help:"Number of authors to generate."`
}

const BATCH_SIZE = 1000

func (c *GenerateCmd) Run(ctx context.Context) error {
	database := pw.GetDB(ctx)

	bus := events.Start(ctx, database)
	defer bus.Shutdown()
	ctx = db.WithBus(ctx, bus)

	projectName := c.Name

	project := db.Project{
		Name:                 projectName,
		Linkname:             projectName,
		Listid:               projectName,
		Listemail:            fmt.Sprintf("%s@%s.example.com", projectName, projectName),
		WebURL:               "",
		ScmURL:               "",
		WebScmURL:            "",
		ListArchiveURL:       "",
		SubjectMatch:         "",
		CommitURLFormat:      "",
		ListArchiveURLFormat: "",
	}
	_, err := database.NewInsert().Model(&project).Exec(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("Created project %q (id=%d)\n", project.Name, project.ID)

	var states []db.State
	err = database.NewSelect().Model(&states).Scan(ctx)
	if err != nil {
		return err
	}

	var tags []db.Tag
	err = database.NewSelect().Model(&tags).Scan(ctx)
	if err != nil {
		return err
	}

	var authors []db.Person
	for i := range c.Authors {
		personName := fmt.Sprintf("person_%s_%d", projectName, i)

		person := db.Person{
			Email: fmt.Sprintf("%s@%s.example.com", personName, projectName),
			Name:  &personName,
		}
		_, err := database.NewInsert().Model(&person).Exec(ctx)
		if err != nil {
			return err
		}

		authors = append(authors, person)
	}

	series_batches := c.Series / BATCH_SIZE
	last_batch_size := c.Series % BATCH_SIZE
	if last_batch_size != 0 {
		series_batches += 1
	}

	for sb := range series_batches {
		var batch_size int
		if sb == series_batches-1 && last_batch_size != 0 {
			batch_size = last_batch_size
		} else {
			batch_size = BATCH_SIZE
		}

		// Create covers

		var batch_covers []db.Cover
		for b := range batch_size {
			batch_covers = append(
				batch_covers,
				randCover(project, authors[rand.Int()%len(authors)], sb*BATCH_SIZE+b),
			)
		}

		_, err := database.NewInsert().Model(&batch_covers).Returning("id").Exec(ctx)
		if err != nil {
			return err
		}

		// Create series

		var batch_series []db.Series
		for _, cover := range batch_covers {
			batch_series = append(batch_series, randSeries(project, cover))
		}
		_, err = database.NewInsert().Model(&batch_series).Returning("id").Exec(ctx)
		if err != nil {
			return err
		}

		// Create patches for each series

		var batch_patches []db.Patch
		for s, series := range batch_series {
			for p := range series.Total {
				name := fmt.Sprintf("series %d patch %d", sb*BATCH_SIZE+s, p)
				batch_patches = append(
					batch_patches,
					randPatch(project, series, states[rand.Int()%len(states)], name, p),
				)
			}
		}

		err = database.NewInsert().Model(&batch_patches).Returning("id").Scan(ctx)
		if err != nil {
			return err
		}

		// Create tags and checks

		var batch_tags []db.PatchTag
		var batch_checks []db.Check
		for _, patch := range batch_patches {
			for t := range len(tags) {
				if rand.Int()%(t+2) != t {
					continue
				}

				batch_tags = append(batch_tags, db.PatchTag{
					PatchID: patch.ID,
					TagID:   tags[t].ID,
					Count:   rand.Int() % 10,
				})
			}

			for range rand.Int() % 10 {
				batch_checks = append(batch_checks, randCheck(patch))
			}
		}

		err = database.NewInsert().Model(&batch_tags).Returning("").Scan(ctx)
		if err != nil {
			return err
		}

		err = database.NewInsert().Model(&batch_checks).Returning("").Scan(ctx)
		if err != nil {
			return err
		}

		fmt.Printf("\rgenerated series %d/%d", sb*BATCH_SIZE+batch_size, c.Series)
	}
	fmt.Printf("\n")

	return nil
}

const charset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

func randString(length uint32) string {
	res := make([]byte, length)
	for i := range res {
		res[i] = charset[rand.Intn(len(charset))]
	}
	return string(res)
}

func randTextBlock(lines uint32, lineLen uint32) string {
	result := make([]byte, (lineLen+1)*lines)
	for i := range lines {
		line := randString(lineLen)
		copy(result[i*lineLen+1:], []byte(line))
		result[(i+1)*lineLen] = '\n'
	}
	return string(result)
}

func randCover(project db.Project, submitter db.Person, num int) db.Cover {
	name := fmt.Sprintf("cover %d", num)
	content := randTextBlock((rand.Uint32()%500)+10, 72)
	headers := randTextBlock((rand.Uint32()%40)+10, 72)
	msgid := fmt.Sprintf("<%s@%s.example.com>", randString(15), project.Name)

	return db.Cover{
		Msgid:       msgid,
		Date:        time.Now(),
		Headers:     headers,
		SubmitterID: submitter.ID,
		Content:     &content,
		ProjectID:   project.ID,
		Name:        name,
	}
}

func randSeries(project db.Project, cover db.Cover) db.Series {
	patches := rand.Int() % 20

	return db.Series{
		ProjectID:     &project.ID,
		CoverLetterID: &cover.ID,
		Name:          &cover.Name,
		Date:          time.Now(),
		SubmitterID:   cover.SubmitterID,
		Version:       0,
		Total:         patches,

		ReceivedTotal: patches,
		ReceivedAll:   true,
	}
}

func randPatch(project db.Project, series db.Series, state db.State, name string, num int) db.Patch {
	msgid := fmt.Sprintf("<%s@%s.example.com>", randString(15), project.Name)
	content := randTextBlock((rand.Uint32()%500)+10, 72)
	headers := randTextBlock((rand.Uint32()%40)+10, 72)
	diff := ""
	hash := randString(32)

	return db.Patch{
		Msgid:       msgid,
		Date:        time.Now(),
		Headers:     headers,
		SubmitterID: series.SubmitterID,
		Content:     &content,
		ProjectID:   project.ID,
		Name:        name,
		Diff:        &diff,
		StateID:     &state.ID,
		Archived:    (rand.Int() % 2) == 0,
		Hash:        &hash,
		SeriesID:    &series.ID,
		Number:      &num,
	}
}

func randCheck(patch db.Patch) db.Check {
	return db.Check{
		PatchID:     patch.ID,
		Date:        time.Now(),
		State:       db.CheckState(rand.Int() % 4),
		TargetURL:   "",
		Context:     fmt.Sprintf("check_%d", rand.Int()%5),
		Description: "",
	}
}
