.. Patchwork - automated patch tracking system
.. Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
..
.. SPDX-License-Identifier: GPL-2.0-or-later


Clients
=======

A number of clients are available for interacting with Patchwork's REST API.


git-pw
------

The `git-pw` application can be used to integrate Patchwork with Git. The
`git-pw` application relies on the REST API and can be used to list, download
and apply series, bundles and individual patches.

More information on `git-pw`, including installation and usage instructions, can
be found in the documentation__ and the `GitHub repo`__.

__ https://git-pw.readthedocs.io/
__ https://github.com/getpatchwork/git-pw/


VSCode-Patchwork
----------------

The *Patchwork* VSCode plugin can be used to integrate Patchwork with VSCode.
This plugin relies on the REST API and can be used to view both patches and
series and to apply them locally. You can also browse patches and series and
look at replies.

More information on the *Patchwork* VSCode plugin can be found on the `VSCode
Marketplace`__ and the `GitHub repo`__.

__ https://marketplace.visualstudio.com/items?itemName=florent-revest.patchwork
__ https://github.com/FlorentRevest/vscode-patchwork


snowpatch
---------

The *snowpatch* application is a bridge between Patchwork and the Jenkins
continuous integration automation server. It monitors the REST API for incoming
patches, applies them on top of an existing git tree, triggers appropriate
builds and test suites, and reports the results back to Patchwork.

Find out more about `snowpatch` at its `GitHub repo`__.

__ https://github.com/ruscur/snowpatch
