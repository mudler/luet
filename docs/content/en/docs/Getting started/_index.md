---
title: "Getting Started"
linkTitle: "Getting Started"
weight: 1
description: >
  First steps with Luet
---


## Prerequisites

No dependencies. For building packages [see the Build Packages section](/docs/concepts/overview/build_packages/)

## Get Luet  

### From release

Just grab a release from [the release page on GitHub](https://github.com/mudler/luet/releases). The binaries are statically compiled.

Or you can install Luet also with a single command:

```bash
curl https://luet.io/install.sh | sudo sh
``` 

### Building Luet from source

Requirements:

- [Golang](https://golang.org/) installed in your system.
- make


```bash
$> git clone https://github.com/mudler/luet
$> cd luet
$> make build # or just go build
```

## Install it as a system package

In the following section we will see how to install luet with luet itself. We will use a transient luet version that we are going to throw away right after we install it in the system.

```bash
# Get a luet release. It will be used to install luet in your system
wget https://github.com/mudler/luet/releases/download/0.8.3/luet-0.8.3-linux-amd64 -O luet
chmod +x luet

# Creates the luet configuration file and add the luet-index repository.
# The luet-index repository contains a collection of repositories which are 
# installable and tracked in your system as standard packages.
cat > .luet.yaml <<EOF
repositories:
- name: "mocaccino-repository-index"
  description: "MocaccinoOS Repository index"
  type: "http"
  enable: true
  cached: true
  priority: 1
  urls:
  - "https://raw.githubusercontent.com/mocaccinoOS/repository-index/gh-pages"
EOF

# Install the official luet repository to get always the latest luet version
./luet install repository/luet

# Install luet (with luet) in your system
./luet install system/luet

# Remove the temporary luet used for bootstrapping
rm -rf luet

# Copy over the config file to your system
mkdir -p /etc/luet
mv .luet.yaml /etc/luet/luet.yaml
```

## Configuration

Luet stores its configuration files in `/etc/luet`. If you wish to override its default settings, create a file `/etc/luet/luet.yaml`.

An example of a configuration file can be found [here](https://github.com/mudler/luet/blob/master/contrib/config/luet.yaml).

There are a bunch of configuration settings available, but the most relevant are:

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
```

To learn more about how to configure luet, [see the configuration section](/docs/concepts/overview/configuration/)