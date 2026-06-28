// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package mail

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/emersion/go-message/mail"

	"github.com/getpatchwork/patchwork/pkg/db"
	"github.com/getpatchwork/patchwork/pkg/log"
)

func (p *parser) getOrCreateAuthor() (*db.Person, error) {
	if strings.EqualFold(p.from.Address, p.project.Listemail) {
		orig := GetOriginalSender(p.header, p.from)
		log.Debugf("DMARC unmangle: %s -> %s", p.from.Address, orig.Address)
		p.from = orig
	}

	person, err := p.db.GetOrCreatePerson(p.from.Address, p.from.Name)
	if err != nil {
		return nil, fmt.Errorf("get or create person: %w", err)
	}

	var name string
	if person.Name != nil {
		name = *person.Name
	}
	log.Debugf("author: %s <%s> (id=%d)", name, person.Email, person.ID)
	return person, nil
}

var (
	listIdAngleRe = regexp.MustCompile(`.*<([^>]+)>.*`)
	listIdRe      = regexp.MustCompile(`^([\S]+)$`)
	listIDHeaders = []string{"List-ID", "X-Mailing-List", "X-list"}
)

func (p *parser) resolveProject() error {
	// if listid was provided explicitly, use it directly
	if p.listid != "" {
		log.Debugf("using explicit list-id=%q", p.listid)
		return p.resolveProjectByIDAndSubject()
	}

	for _, headerName := range listIDHeaders {
		txt, _ := p.header.Text(headerName)
		if txt == "" {
			continue
		}

		var lid string
		for _, re := range []*regexp.Regexp{listIdAngleRe, listIdRe} {
			if m := re.FindStringSubmatch(txt); m != nil {
				lid = m[1]
				break
			}
		}
		if lid == "" {
			continue
		}

		log.Debugf("found list-id=%q from %s header", lid, headerName)
		p.listid = lid

		if p.resolveProjectByIDAndSubject() == nil {
			break
		}
	}

	return nil
}

func (p *parser) resolveProjectByIDAndSubject() error {
	projects, err := p.db.GetProjectByListID(p.listid)
	if err != nil {
		return err
	}

	for i := range projects {
		proj := &projects[i]
		if proj.SubjectMatch == "" {
			p.project = proj
		} else {
			re, err := regexp.Compile("(?mi)" + proj.SubjectMatch)
			if err != nil {
				continue
			}
			if re.MatchString(p.subject) {
				p.project = proj
				break
			}
		}
	}

	return nil
}

func (p *parser) findState() (*db.State, error) {
	if name := p.header.Get("X-Patchwork-State"); name != "" {
		state, err := p.db.GetStateByName(name)
		if err == nil {
			log.Debugf("explicit state: %s (id=%d)", state.Name, state.ID)
			return state, nil
		}
		log.Debugf("requested state %q not found, using default", name)
	}
	return p.db.GetDefaultState()
}

func (p *parser) findDelegate() *int {
	addr, _ := mail.ParseAddress(p.header.Get("X-Patchwork-Delegate"))
	if addr != nil {
		userID, err := p.db.GetUserIDByEmail(addr.Address)
		if err == nil {
			log.Debugf("explicit delegate: %s (user=%d)", addr.Address, userID)
			return db.Ptr(userID)
		}
		log.Debugf("requested delegate %s not found", addr.Address)
	}

	if p.content.diff == "" {
		return nil
	}

	filenames := FindFilenames(p.content.diff)
	if len(filenames) == 0 {
		return nil
	}

	rules, err := p.db.ListDelegationRulesByProject(p.project.ID)
	if err != nil || len(rules) == 0 {
		return nil
	}

	d := FindDelegateByFilename(rules, filenames)
	if d != nil {
		log.Debugf("delegate from filenames: user=%d", *d)
	}
	return d
}
