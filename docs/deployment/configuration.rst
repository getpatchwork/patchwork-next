.. Patchwork - automated patch tracking system
.. Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
..
.. SPDX-License-Identifier: GPL-2.0-or-later

Configuration
=============

Patchwork is configured through a TOML file. The default configuration can be
generated with::

    $ pw config

Configuration files are loaded in the following order, with later files
overriding earlier ones:

1. ``/etc/patchwork.toml``
2. ``patchwork.toml`` (current directory)
3. ``$PATCHWORK_TOML`` (environment variable)

Command-line flags take precedence over all configuration files.


Settings Reference
------------------

``[database]``
~~~~~~~~~~~~~~

``url``
  Database connection URL. Supported schemes:

  - ``postgres://user:pass@host/dbname?sslmode=disable``
  - ``mysql://user:pass@host/dbname``
  - ``sqlite:///path/to/file.db``

``auto-sync``
  Automatically run pending migrations when the HTTP server starts.
  Default: ``false``.

``[http]``
~~~~~~~~~~

``listen``
  HTTP listen address. Default: ``127.0.0.1:8080``.

``base-url``
  The public base URL of the Patchwork instance, used for generating links in
  emails and API responses. Example: ``https://patchwork.example.com``.

``custom-css``
  Path to a custom CSS file. It is served after the built-in stylesheet,
  allowing you to override any default styles. The file is read once at startup.

``nav-html``
  Path to an HTML file whose content is inserted in the page header, after the
  navigation bar. Can be used to display a logo, additional links, or a banner.
  The file is read once at startup.

``footer-html``
  Path to an HTML file whose content is inserted in the page footer. Can be used
  for legal notices or organization-specific links. The file is read once at
  startup.

``[ingress]``
~~~~~~~~~~~~~

``listen``
  SMTP listen address for the ingress daemon. Default: ``127.0.0.1:2525``.

``[smtp]``
~~~~~~~~~~

Outgoing mail configuration for notifications.

``encryption``
  SMTP encryption mode. One of ``none``, ``starttls``, ``tls``.
  Default: ``none``.

``host``
  SMTP server hostname. Default: ``localhost``.

``port``
  SMTP server port. Default: ``25``.

``user``
  SMTP authentication username. Leave empty for unauthenticated delivery.

``password``
  SMTP authentication password.

``from``
  Sender email address for outgoing notifications.
  Default: ``patchwork@localhost``.


Global Flags
------------

``-S``, ``--syslog``
  Redirect logging to syslog instead of stderr.
