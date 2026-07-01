# Debian package contract

This document defines the local Debian package layout, lifecycle behavior, validation targets, and inspection gates for podlaz.

The package installs the CLI, daemon, service unit, sysusers configuration, shell completions, man pages, and documentation under Debian/FHS locations.

## Service install behavior

Installing the package must make the local `podlazd` daemon available for normal first-run CLI usage without starting a VPN connection or changing host networking.

Package installation:

- creates or declares packaged service identities through `systemd-sysusers` when available;
- unmasks and enables `podlazd.service` through Debian systemd helper tooling when available;
- requests daemon startup through Debian systemd invocation helper tooling when available;
- does not start Xray by itself;
- does not create TUN devices;
- does not change routes, DNS, nftables, firewall rules, or resolver files.
