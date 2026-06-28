// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package events

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/emersion/go-message/mail"
	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"

	"github.com/getpatchwork/patchwork/pkg/config"
	"github.com/getpatchwork/patchwork/pkg/log"
)

func SendEmail(cfg *config.SMTPConfig, to, subject, body string, extraHeaders map[string]string) error {
	from, err := mail.ParseAddress(cfg.From)
	if err != nil {
		return fmt.Errorf("from: %q %w", from, err)
	}
	rcpt, err := mail.ParseAddress(to)
	if err != nil {
		return fmt.Errorf("to: %q %w", to, err)
	}

	var h mail.Header
	h.SetAddressList("From", []*mail.Address{from})
	h.SetAddressList("To", []*mail.Address{rcpt})
	h.SetSubject(subject)
	h.SetDate(time.Now())
	_, domain, _ := strings.Cut(from.Address, "@")
	_ = h.GenerateMessageIDWithHostname(domain)
	h.SetContentType("text/plain", map[string]string{"charset": "utf-8"})

	for k, v := range extraHeaders {
		h.Set(k, v)
	}

	var buf bytes.Buffer
	w, err := mail.CreateSingleInlineWriter(&buf, h)
	if err != nil {
		return fmt.Errorf("writer: %w", err)
	}
	if _, err = w.Write([]byte(body)); err != nil {
		w.Close()
		return fmt.Errorf("body write: %w", err)
	}
	if err = w.Close(); err != nil {
		return fmt.Errorf("body close: %w", err)
	}

	log.Infof("sending email to=%s subject=%q", to, subject)

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	var c *smtp.Client

	if cfg.Encryption == "" {
		switch cfg.Port {
		case 25:
			cfg.Encryption = "none"
		case 465:
			cfg.Encryption = "tls"
		default:
			cfg.Encryption = "starttls"
		}
	}
	switch cfg.Encryption {
	case "none":
		c, err = smtp.Dial(addr)
	case "tls":
		c, err = smtp.DialTLS(addr, nil)
	case "starttls":
		c, err = smtp.DialStartTLS(addr, nil)
	default:
		panic("invalid encryption " + cfg.Encryption)
	}
	if err != nil {
		return err
	}
	defer func() {
		if e := c.Close(); e != nil {
			log.Errorf("close: %v", e)
		}
	}()

	if cfg.User != "" {
		var auth sasl.Client
		switch {
		case c.SupportsAuth(sasl.Plain):
			auth = sasl.NewPlainClient("", cfg.User, cfg.Password)
		case c.SupportsAuth(sasl.Login):
			auth = sasl.NewLoginClient(cfg.User, cfg.Password)
		default:
			return fmt.Errorf("server does not support AUTH")
		}
		if err = c.Auth(auth); err != nil {
			return fmt.Errorf("auth: %T %w", auth, err)
		}
	}

	return c.SendMail(from.Address, []string{rcpt.Address}, &buf)
}
