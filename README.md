Patchwork
=========

**Patchwork** is a patch tracking system for community-based projects. It is
intended to make the patch management process easier for both the project's
contributors and maintainers, leaving time for the more important (and more
interesting) stuff.

Patches that have been sent to a mailing list are "caught" by the system, and
appear on a web page. Any comments posted that reference the patch are appended
to the patch page too. The project's maintainer can then scan through the list
of patches, marking each with a certain state, such as Accepted, Rejected or
Under Review. Old patches can be sent to the archive or deleted.

Currently, Patchwork is being used for a number of open-source projects, mostly
subsystems of the Linux kernel. Although Patchwork has been developed with the
kernel workflow in mind, the aim is to be flexible enough to suit the majority
of community projects.

> [!NOTE]
>
> This is the 4.0 rewrite of Patchwork in Go, replacing the Django/Python
> codebase with a single statically compiled binary.

## Quickstart

Build and install:

    make
    sudo make install

Generate a configuration file and edit it:

    pw config > /etc/patchwork.toml
    vim /etc/patchwork.toml

Initialize the database, create an admin user and a project:

    pw db sync
    pw admin user create --admin -u admin -e admin@example.com
    pw admin project create \
        -n "My Project" -l my-project \
        -i my-project.example.com -e patches@example.com

Start the services:

    pw http &
    pw ingress &

The web interface is now available at http://127.0.0.1:8080 and the SMTP
ingress daemon listens on 127.0.0.1:2525.

For production deployment with systemd, nginx and Postfix, see the
[full documentation](https://getpatchwork.github.io/patchwork-next/).
