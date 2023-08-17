title: "Configuration"
linkTitle: "Configuration
weight: 2
description: >
    Configuring Luet
---

### General

```yaml
general:
  # Define max concurrency processes. Default is based of arch: runtime.NumCPU()
  concurrency: 1
  # Enable Debug. If debug is active spinner is disabled.
  debug: false
  # Show output of build execution (docker, img, etc.)
  show_build_output: false
  # Define spinner ms
  spinner_ms: 200
  # Define spinner charset. See https://github.com/briandowns/spinner
  spinner_charset: 22
  # Enable warnings to exit
  fatal_warnings: false
  # Try extracting tree/packages with the same ownership as exists in the archive (default for superuser).
  same_owner: false
```

### Images

After the building of the packages, you can apply arbitrary images on top using the `images` stanza. This is useful if you need to pin a package to a specific version.

```yaml
images:
  - quay.io/kairos/packages:kairos-agent-system-2.1.12
```

### Logging

```yaml
logging:
  # Enable loggging to file (if path is not empty)
  enable_logfile: false
  #  Leave empty to skip logging to file.
  path: "/var/log/luet.log"
  # Set logging level: error|warning|info|debug
  level: "info"
  #  Enable JSON log format instead of console mode.
  json_format: false.
  #  Disable/Enable color
  color: true
  #  Enable/Disable emoji
  enable_emoji: true
```

### Repositories configurations directories.

```yaml
# Define the list of directories where luet
# try for files with .yml extension that define
# luet repository.
repos_confdir:
  - /etc/luet/repos.conf.d
```

### Finalizer Environment Variables

```yaml
finalizer_envs:
 - key: "BUILD_ISO"
   value: "1"
```

### Repositories

To add repositories, you can either add a `repositories` stanza in your `/etc/luet/luet.yaml` or either add one or more yaml files in `/etc/luet/repos.conf.d/`.

#### Configuring repositories in the main configuration file

```yaml
logging:
  color: true # Enable/Disable colored output
  enable_emoji: true # Enable/Disable emoji from output
general:
  debug: false # Enable/Disable debug
system:
  rootfs: "/" # What's our rootfs. Luet can install packages outside of "/"
  database_path: "/var/db/luet" # Where to store DB files
  database_engine: "boltdb"
  tmpdir_base: "/var/tmp/luet" # The temporary directory to be used
repositories:
- name: "some-repository-name" # Repository name
  description: "A beautiful description"
  type: "http" # Repository type, disk or http are supported (disk for local path)
  enable: true # Enable/Disable repo
  cached: true # Enable cache for repository
  priority: 3 # Cache priority
  urls: # Repository URLs
    - "...."
```

#### Using different files to configure repositories

In the main configuration file you can specify the directory where all repositories are configured:

```yaml
repos_confdir:
  - /etc/luet/repos.conf.d
```
 
Then add a file inside `/etc/luet/repos.conf.d/example.yaml` with your configuration, e.g.:

```yaml
name: "..." # Repository name
description: "..."
type: "http" # Repository type, disk or http are supported (disk for local path)
enable: true # Enable/Disable repo
cached: true # Enable cache for repository
priority: 3 # Cache priority
urls: # Repository URLs
  - "..."
```

There is available a [collection of repositories](https://packages.mocaccino.org/repository-index), which is containing a list of repositories that can be installed in the system with `luet install`.

If you installed Luet from the curl command, you just need to run `luet search repository` to see a list of all the available repository, and you can install them singularly by running `luet install repository/<name>`. Otherwise, add the repository stanzas you need to `/etc/luet/luet.yaml`.

#### Config protect configuration files directories.

```yaml
# Define the list of directories where load
# configuration files with the list of config
# protect paths.
config_protect_confdir:
  - /etc/luet/config.protect.d
# Ignore rules defined on
# config protect confdir and packages
# annotation.
config_protect_skip: false
# The paths used for load repositories and config
# protects are based on host rootfs.
# If set to false rootfs path is used as prefix.
config_from_host: true
```

### Solver Parameter Configuration

```yaml
solver:
  # Solver strategy to solve possible conflicts during depedency
  # solving. Defaults to empty (none). Available: qlearning
  type: ""
  # Solver agent learning rate. 0.1 to 1.0
  rate: 0.7
  # Learning discount factor.
  discount: 1.0
  # Number of overall attempts that the solver has available before bailing out.
  max_attempts: 9000
```

### System

```yaml
system:
  # Rootfs path of the luet system. Default is /.
  # A specific path could be used for test installation to
  # a chroot environment.
  rootfs: "/"
  # Database engine used for luet database.
  # Supported values: boltdb|memory
  database_engine: boltdb
  # Database path directory where store luet database.
  # The path is appended to rootfs option path.
  database_path: "/var/cache/luet"
  # Define the tmpdir base directory where luet store temporary files.
  # Default $TMPDIR/tmpluet
  tmpdir_base: "/tmp/tmpluet"
```
