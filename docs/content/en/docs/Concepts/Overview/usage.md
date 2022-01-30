---
title: "CLI usage"
linkTitle: "CLI usage"
weight: 3
date: 2019-12-14
description: >
  How to install packages, manage repositories, ...
---


## Installing a package

To install a package with `luet`, simply run:

```bash

$ luet install <package_name>

```

To relax dependency constraints and avoid auto-upgrades, add the `--relax` flag:

```bash
$ luet install --relax <package name>
```

To install only the package without considering the deps, add the `--nodeps` flag:

```bash
$ luet install --nodeps <package name>
```

To install only package dependencies, add the `--onlydeps` flag:

```bash
$ luet install --onlydeps <package name>
```

To only download packages, without installing them use the `--download-only` flag:

```bash
$ luet install --download-only <package name>
```

## Uninstalling a package

To uninstall a package with `luet`, simply run:

```bash

$ luet uninstall <package_name>

```

## Upgrading the system

To upgrade your system, simply run:

```bash
$ luet upgrade
```

## Refreshing repositories

Luet automatically syncs repositories definition on the machine when necessary, but it avoids to sync up in a 24h range. In order to refresh the repositories manually, run:

```bash
$ luet repo update
```

## Searching a package

To search a package:

```bash

$ luet search <regex>

```

To search a package and display results in a table:

```bash

$ luet search --table <regex>

```

To look into the installed packages:

```bash

$ luet search --installed <regex>

```

Note: the regex argument is optional

## Search file belonging to packages

```bash
$ luet search --file <file_pattern>
```

### Search output

Search can return results in the terminal in different ways: as terminal output, as json or as yaml.

#### JSON

```bash

$ luet search --json <regex>

```

#### YAML

```bash

$ luet search --yaml <regex>

```

#### Tabular


```bash

$ luet search --table <regex>

```

## Quiet luet output

Luet output is verbose by default and colourful, however will try to adapt to the terminal, based on which environment is executed (as a service, in the terminal, etc.)

You can quiet `luet` output with  the `--quiet` flag or `-q` to have a more compact output in all the commands.