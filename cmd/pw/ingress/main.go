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
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/emersion/go-mbox"
	"github.com/emersion/go-message"
	"github.com/emersion/go-smtp"
	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/cmd/pw/pw"
	"github.com/getpatchwork/patchwork/pkg/config"
	"github.com/getpatchwork/patchwork/pkg/db"
	"github.com/getpatchwork/patchwork/pkg/db/migrations"
	"github.com/getpatchwork/patchwork/pkg/events"
	"github.com/getpatchwork/patchwork/pkg/log"
	"github.com/getpatchwork/patchwork/pkg/mail"
)

type CLI struct {
	Stdin  bool   `short:"i" help:"Read one email from stdin and exit."`
	Mbox   bool   `short:"m" help:"Read all emails in mbox format from stdin."`
	ListID string `short:"l" help:"Force List-ID value instead of reading it from email headers."`
}

func (c *CLI) Run(ctx *pw.Context) error {
	if ctx.Config.Database.AutoSync {
		if err := migrations.RunMigrations(ctx, ctx.DB); err != nil {
			return err
		}
	} else if err := migrations.CheckSchemaVersion(ctx, ctx.DB); err != nil {
		return err
	}

	bus := events.Start(ctx, ctx.DB)
	defer bus.Shutdown()
	ctx.Context = db.WithBus(ctx.Context, bus)

	if c.Stdin || c.Mbox {
		var dupErr *mail.DuplicateMailError
		var err error

		if c.Mbox {
			var msg io.Reader
			reader := mbox.NewReader(os.Stdin)
			for {
				msg, err = reader.NextMessage()
				if err != nil {
					break
				}
				err = mail.ParseMail(ctx, ctx.DB, msg, c.ListID)
				if errors.As(err, &dupErr) {
					log.Debugf("ignoring %s", err)
				} else if err != nil {
					break
				}
			}
		} else {
			err = mail.ParseMail(ctx, ctx.DB, os.Stdin, c.ListID)
		}
		if errors.As(err, &dupErr) {
			log.Debugf("ignoring %s", err)
		} else if err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("smtp: %w", err)
		}
		return nil
	}

	be := &backend{
		ctx:      ctx.Context,
		database: ctx.DB,
		cfg:      ctx.Config,
		listID:   c.ListID,
	}

	sock, srv, err := startSMTPServer(ctx.Config, be)
	if err != nil {
		return fmt.Errorf("smtp: %w", err)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Noticef("patchwork %s listening on smtp://%s", ctx.Version, sock.Addr())
		if e := srv.Serve(sock); e != nil && !errors.Is(e, net.ErrClosed) {
			err = fmt.Errorf("serve: %w", e)
			done <- syscall.SIGCHLD
		}
	}()

	sig := <-done
	if sig != syscall.SIGCHLD {
		log.Noticef("received signal %v, shutting down", sig)
		timeout, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if e := srv.Shutdown(timeout); e != nil {
			err = fmt.Errorf("shutdown: %w", e)
		}
	}

	return err
}

func startSMTPServer(cfg *config.Config, be smtp.Backend) (net.Listener, *smtp.Server, error) {
	s := smtp.NewServer(be)
	s.Addr = cfg.Ingress.Listen
	s.Domain = "localhost"
	s.ReadTimeout = 30 * time.Second
	s.WriteTimeout = 30 * time.Second
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
	ctx      context.Context
	database *bun.DB
	cfg      *config.Config
	listID   string
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
		s.backend.ctx, s.backend.database,
		bytes.NewReader(data), s.backend.listID,
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
