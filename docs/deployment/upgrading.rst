.. Patchwork - automated patch tracking system
.. Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
..
.. SPDX-License-Identifier: GPL-2.0-or-later

Upgrading
=========

This document covers upgrading between Patchwork 4.x releases. For migrating
from the Django-based Patchwork (3.x and earlier) to 4.0, see
:doc:`migration`.

Before Upgrading
----------------

Always back up your database before upgrading:

.. code-block:: console

   $ pg_dump patchwork > patchwork-backup.sql

Upgrade Steps
-------------

1. Install the new ``pw`` binary (e.g. ``make && sudo make install``).

2. Run pending database migrations:

   .. code-block:: console

      $ pw db sync

3. Restart the services:

   .. code-block:: console

      $ sudo systemctl restart pw-http pw-ingress

That's it. There are no static files to collect, no Python dependencies to
update, and no virtual environments to rebuild.
