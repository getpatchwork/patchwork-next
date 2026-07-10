.. Patchwork - automated patch tracking system
.. Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
..
.. SPDX-License-Identifier: GPL-2.0-or-later

Configuration
=============

Patchwork is configured through a TOML file. The default configuration can be
generated with:

.. code-block:: console

    $ pw config

Configuration files are loaded in the following order, with later files
overriding earlier ones:

1. `/etc/patchwork.toml`
2. `patchwork.toml` (current directory)
3. `$PATCHWORK_TOML` (environment variable)

Command-line flags take precedence over all configuration files.


Settings Reference
------------------

.. confval:: syslog
   :type: bool
   :default: `false`

   Redirect logging to syslog instead of stderr.

.. confval:: [database].url
   :type: string
   :default: `""`

   Database connection URL. Supported schemes:

   - `postgres://user:pass@host/dbname?sslmode=disable`
   - `mysql://user:pass@host/dbname`
   - `sqlite:///path/to/file.db`

   .. important::

      The `user` and `pass` values must be URL-encoded. Use the `pw config
      url` command to generate valid connection URLs:

      .. code-block:: console

         $ ./pw config url
         Database type (postgres, mysql, sqlite) [postgres]: mysql
         Host [localhost]:
         Port [3306]:
         Database name [patchwork]: patchwork4
         Username [patchwork]:
         Password: p@tch/w0rk!

         url = "mysql://patchwork:p%40tch%2Fw0rk%21@localhost/patchwork4"

.. confval:: [database].auto-sync
   :type: bool
   :default: `false`

   Automatically run pending migrations when the HTTP server starts.

.. confval:: [http].listen
   :type: string
   :default: `"127.0.0.1:8080"`

   HTTP listen address.

.. confval:: [http].base-url
   :type: string
   :default: `""`

   The public base URL of the Patchwork instance, used for generating links in
   emails and API responses. Example: `https://patchwork.example.com`.

.. confval:: [http].custom-css
   :type: string
   :default: `""`

   Path to a custom CSS file. It is served after the built-in stylesheet,
   allowing you to override any default styles. The file is read once at startup.

.. confval:: [http].nav-html
   :type: string
   :default: `""`

   Path to an HTML file whose content is inserted in the page header, after the
   navigation bar. Can be used to display a logo, additional links, or a banner.
   The file is read once at startup.

.. confval:: [http].footer-html
   :type: string
   :default: `""`

   Path to an HTML file whose content is inserted in the page footer. Can be used
   for legal notices or organization-specific links. The file is read once at
   startup.

.. confval:: [ingress].listen
   :type: string
   :default: `"127.0.0.1:2525"`

   SMTP listen address for the ingress daemon.

.. confval:: [smtp].encryption
   :type: enum (`none`, `starttls`, `tls`)
   :default: `none`

   SMTP encryption mode.

.. confval:: [smtp].host
   :type: string
   :default: `"localhost"`

   SMTP server hostname.

.. confval:: [smtp].port
   :type: int
   :default: `25`

   SMTP server port.

.. confval:: [smtp].user
   :type: string
   :default: `""`

   SMTP authentication username. Leave empty for unauthenticated delivery.

.. confval:: [smtp].password
   :type: string
   :default: `""`

   SMTP authentication password.

.. confval:: [smtp].from
   :type: string
   :default: `"patchwork@localhost"`

   Sender email address for outgoing notifications.
