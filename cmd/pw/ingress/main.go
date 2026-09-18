// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package ingress

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/emersion/go-mbox"
	"github.com/emersion/go-message"
	"github.com/emersion/go-smtp"

	"github.com/getpatchwork/patchwork/cmd/pw/pw"
	"github.com/getpatchwork/patchwork/pkg/db"
	"github.com/getpatchwork/patchwork/pkg/db/migrations"
	"github.com/getpatchwork/patchwork/pkg/events"
	"github.com/getpatchwork/patchwork/pkg/log"
	"github.com/getpatchwork/patchwork/pkg/mail"
)

type CLI struct {
	Stdin     bool   `short:"i" help:"Read one email from stdin and exit."`
	Mbox      bool   `short:"m" help:"Read all emails in mbox format from stdin."`
	ListID    string `short:"l" help:"Force List-ID value instead of reading it from email headers."`
	Anonymize bool   `help:"Partially anonymize messages to provided list. ListID has to be specified."`
}

func (c *CLI) Run(ctx context.Context) error {
	cfg := pw.GetConfig(ctx)
	database := pw.GetDB(ctx)

	if cfg.Database.AutoSync {
		if err := migrations.RunMigrations(ctx, database); err != nil {
			return err
		}
	} else if err := migrations.CheckSchemaVersion(ctx, database); err != nil {
		return err
	}

	bus := events.Start(ctx, database)
	defer bus.Shutdown()
	ctx = db.WithBus(ctx, bus)

	if c.Anonymize && c.ListID == "" {
		return fmt.Errorf("anonymize requires listid")
	}

	if c.Stdin || c.Mbox {
		var dupErr *mail.DuplicateMailError
		var parseErr *mail.ParseError
		var err error

		if c.Mbox {
			var msg io.Reader
			reader := mbox.NewReader(os.Stdin)
			for {
				msg, err = reader.NextMessage()
				if err != nil {
					break
				}
				err = mail.ParseMail(ctx, database, msg, c.Anonymize, c.ListID)
				if errors.As(err, &dupErr) || errors.As(err, &parseErr) {
					log.Debugf("ignoring %s", err)
				} else if err != nil {
					break
				}
			}
		} else {
			err = mail.ParseMail(ctx, database, os.Stdin, c.Anonymize, c.ListID)
		}
		if errors.As(err, &dupErr) || errors.As(err, &parseErr) {
			log.Debugf("ignoring %s", err)
		} else if err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("smtp: %w", err)
		}
		return nil
	}

	sock, srv, err := c.startSMTPServer(ctx, c.Anonymize)
	if err != nil {
		return fmt.Errorf("smtp: %w", err)
	}

	log.Noticef("patchwork %s listening on smtp://%s", pw.GetVersion(ctx), sock.Addr())

	unregister := context.AfterFunc(ctx, func() {
		log.Noticef("%s, shutting down", context.Cause(ctx))
		timeout, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if e := srv.Shutdown(timeout); e != nil {
			log.Errorf("shutdown: %v", e)
		}
	})
	defer unregister()

	if err = srv.Serve(sock); err != nil && !errors.Is(err, net.ErrClosed) {
		return fmt.Errorf("serve: %w", err)
	}

	return nil
}

func (c *CLI) startSMTPServer(ctx context.Context, anonymize bool) (net.Listener, *smtp.Server, error) {
	cfg := pw.GetConfig(ctx)

	s := smtp.NewServer(&backend{ctx: ctx, listID: c.ListID, anonymize: anonymize})
	s.Addr = cfg.Ingress.Listen
	s.Domain = "localhost"
	s.ReadTimeout = 30 * time.Second
	s.WriteTimeout = 30 * time.Second
	s.MaxMessageBytes = cfg.Ingress.MaxMessageSize
	s.AllowInsecureAuth = true
	s.EnableSMTPUTF8 = true
	s.LMTP = strings.Contains(cfg.Ingress.Listen, "/")
	s.ErrorLog = log.ErrLogger()

	network := "tcp"
	if s.LMTP {
		network = "unix"
	}
	l, err := net.Listen(network, s.Addr)
	if err != nil {
		return nil, nil, fmt.Errorf("listen: %w", err)
	}
	return l, s, nil
}

type backend struct {
	ctx       context.Context
	listID    string
	anonymize bool
}

func (b *backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	return &session{
		backend: b,
		remote:  c.Conn().RemoteAddr(),
	}, nil
}

type session struct {
	backend *backend
	remote  net.Addr
	from    string
	to      []string
}

func (s *session) Reset() {
	s.from = ""
	s.to = nil
}

func (s *session) Logout() error {
	return nil
}

func (s *session) Mail(from string, opts *smtp.MailOptions) error {
	s.from = from
	return nil
}

func (s *session) Rcpt(to string, opts *smtp.RcptOptions) error {
	s.to = append(s.to, to)
	return nil
}

func (s *session) Data(r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	// check for auto-submitted messages (OOO, auto-replies)
	entity, err := message.Read(bytes.NewReader(data))
	if err != nil {
		if !message.IsUnknownCharset(err) {
			return &smtp.SMTPError{
				Code:         550,
				EnhancedCode: smtp.EnhancedCode{5, 6, 0},
				Message:      fmt.Sprintf("failed to parse message: %v", err),
			}
		}
		log.Noticef("unknown charset: %v", err)
	}

	autoSubmitted := strings.ToLower(entity.Header.Get("Auto-Submitted"))
	switch autoSubmitted {
	case "auto-generated", "auto-replied":
		log.Debugf("ignoring auto-submitted message from=%s msgid=%s",
			s.from, entity.Header.Get("Message-Id"))
		return nil
	}

	log.Infof("message received from=%s to=%s msgid=%s subject=%s",
		s.from, strings.Join(s.to, ","),
		entity.Header.Get("Message-Id"),
		entity.Header.Get("Subject"))

	err = mail.ParseMail(
		s.backend.ctx, pw.GetDB(s.backend.ctx),
		bytes.NewReader(data), s.backend.anonymize, s.backend.listID,
	)
	if err != nil {
		var dupErr *mail.DuplicateMailError
		var parseErr *mail.ParseError
		switch {
		case errors.As(err, &dupErr):
			log.Debugf("ignoring %s", err)
		case errors.As(err, &parseErr):
			// Invalid message, no point in retrying later.
			return &smtp.SMTPError{
				Code:         550,
				EnhancedCode: smtp.EnhancedCode{5, 6, 1},
				Message:      err.Error(),
			}
		case err != nil:
			// Consider any other error during parsing to be non-fatal.
			// Return a 451 error code to ask postfix to retry later.
			return &smtp.SMTPError{
				Code:         451,
				EnhancedCode: smtp.EnhancedCode{4, 3, 0},
				Message:      err.Error(),
			}
		}
	}

	return nil
}
