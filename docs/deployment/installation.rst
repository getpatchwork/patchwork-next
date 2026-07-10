.. Patchwork - automated patch tracking system
.. Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
..
.. SPDX-License-Identifier: GPL-2.0-or-later

Installation
============

This document describes how to deploy a production Patchwork instance. Patchwork
4.0 is a single Go binary that replaces the previous Django/Python deployment.


Requirements
------------

Patchwork requires:

- The `pw` binary (built from source or downloaded from a release)
- A supported database: PostgreSQL (recommended), MySQL/MariaDB, or SQLite
- A mail transfer agent (e.g. Postfix) for receiving patches
- A reverse proxy (e.g. nginx) for TLS termination (optional but recommended)

Building from source requires Go 1.22 or later:

.. code-block:: console

   $ git clone https://github.com/getpatchwork/patchwork.git
   $ cd patchwork
   $ make
   $ sudo make install

Database Setup
--------------

PostgreSQL (Recommended)
~~~~~~~~~~~~~~~~~~~~~~~~

Install PostgreSQL and create a database:

.. code-block:: console

   $ sudo apt-get install -y postgresql
   $ sudo -u postgres createuser -d patchwork
   $ sudo -u postgres createdb -O patchwork patchwork

SQLite
~~~~~~

No setup is needed. Simply point the configuration at a file path:

.. code-block:: toml

   [database]
   url = "sqlite:///var/lib/patchwork/patchwork.db"

.. note::

   SQLite is suitable for small installations and development. For production
   use with multiple concurrent users, PostgreSQL is recommended.


Configuration
-------------

Generate a default configuration file:

.. code-block:: console

   $ pw config > /etc/patchwork.toml

Edit `/etc/patchwork.toml` to set at least the database URL and the HTTP base
URL. See :doc:`configuration` for a full reference of all available settings.

A minimal configuration looks like:

.. code-block:: toml

   [database]
   url = "postgres://patchwork:secret@localhost/patchwork"
   auto-sync = true

   [http]
   listen = "127.0.0.1:8080"
   base-url = "https://patchwork.example.com"

   [ingress]
   listen = "127.0.0.1:2525"

   [smtp]
   host = "localhost"
   port = 25
   from = "patchwork@example.com"


Initialize the Database
-----------------------

Create the database schema and seed default data:

.. code-block:: console

   $ pw db sync


Create an Admin User
--------------------

Create a user account with admin privileges:

.. code-block:: console

   $ pw admin user create --admin -u admin -e admin@example.com

The command will prompt for a password interactively.


Create a Project
----------------

Create your first project:

.. code-block:: console

   $ pw admin project create \
        -n "My Project" \
        -l my-project \
        -i my-project.example.com \
        -e patches@example.com


Running Services
----------------

Patchwork consists of two long-running services:

`pw http`
  The HTTP server exposing the web interface and REST API.

`pw ingress`
  The SMTP daemon that receives emails from your mail transfer agent.

systemd
~~~~~~~

The `make install` target installs systemd unit files. Enable and start the
services:

.. code-block:: console

   $ sudo systemctl daemon-reload
   $ sudo systemctl enable --now pw-http pw-ingress

The unit files are installed to `/usr/lib/systemd/system`. To override settings,
use `systemctl edit`:

.. code-block:: console

   $ sudo systemctl edit pw-http

.. note::

   Both services read configuration from `/etc/patchwork.toml` by default.
   Use the env:`PATCHWORK_TOML` environment variable to specify an alternative
   path.


Reverse Proxy
-------------

A reverse proxy like nginx is recommended for TLS termination and static file
serving. The `make install` target installs a default nginx configuration to
`/etc/nginx/conf.d/patchwork.conf`.

A minimal nginx configuration:

.. code-block:: nginx

   server {
       listen 443 ssl;
       server_name patchwork.example.com;

       ssl_certificate /etc/letsencrypt/live/patchwork.example.com/fullchain.pem;
       ssl_certificate_key /etc/letsencrypt/live/patchwork.example.com/privkey.pem;

       location / {
           proxy_pass http://127.0.0.1:8080;
           proxy_set_header Host $host;
           proxy_set_header X-Real-IP $remote_addr;
           proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
           proxy_set_header X-Forwarded-Proto $scheme;
       }
   }


Incoming Email
--------------

Patchwork needs to receive emails from your mailing list. The recommended
approach is to configure your mail transfer agent to forward messages to the
`pw ingress` SMTP daemon.


.. _ingress-transport:

Postfix with Transport Maps
~~~~~~~~~~~~~~~~~~~~~~~~~~~

This is the recommended setup. Configure Postfix to route mail for your list
domain to the `pw ingress` daemon using transport maps.

Add to `/etc/postfix/main.cf`:

.. code-block:: ini

   transport_maps = lmdb:/etc/postfix/transport

Create `/etc/postfix/transport`:

.. code-block:: bash

   lists.example.com    smtp:127.0.0.1:2525

Build the transport map and reload Postfix:

.. code-block:: console

   $ sudo postmap /etc/postfix/transport
   $ sudo systemctl reload postfix

All mail addressed to `lists.example.com` will now be forwarded to `pw ingress`
over SMTP. No shell scripts, no special user accounts, no database grants.

.. note::

   The `pw ingress` daemon matches incoming emails to projects by their
   `List-ID` header. Make sure the `-e` (list email) and `-i` (list ID)
   values of your project match what your mailing list software produces.

IMAP/POP3
~~~~~~~~~

For simpler setups, you can use a mail retriever like `getmail`__ to download
messages from an inbox and pipe them to Patchwork:

.. code-block:: ini

   [destination]
   type = MDA_external
   path = /usr/bin/pw
   arguments = ("ingress", "--stdin",)

__ http://pyropus.ca/software/getmail/

Manual Import
~~~~~~~~~~~~~

For one-off imports, `pw ingress` can read from stdin:

.. code-block:: console

   $ pw ingress --stdin < email.eml
   $ pw ingress --mbox < archive.mbox

The `--list-id` flag can be used to override the `List-ID` header.


.. _deployment-vcs:

(Optional) VCS Integration
--------------------------

Patchwork can update patch states automatically when commits are pushed.
A `post-receive` Git hook can be configured to mark patches as "accepted" when
their corresponding commits land in the repository.

Refer to the `post-receive.hook` script in the Patchwork source tree for an
example implementation that uses the REST API.


Periodic Cleanup
----------------

Run garbage collection periodically to clean up expired sessions, stale email
confirmations, and inactive users:

.. code-block:: console

   $ pw admin gc

A cron job or systemd timer is recommended (e.g.
`/etc/cron.daily/patchwork-gc`):

.. code-block:: bash

   #!/bin/sh
   exec /usr/bin/pw admin gc
