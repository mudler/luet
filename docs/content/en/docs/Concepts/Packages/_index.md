---
title: "Packages"
linkTitle: "Packages"
weight: 2
description: >
  Package definition syntax
---

A Package in Luet is denoted by a triple (`name`, `category` and `version`), here called *package form* in a `definition.yaml` file in YAML: 

```yaml
name: "awesome"
version: "0.1"
category: "foo"
```

While `category` and `version` can be omitted, the name is required. Note that when refering to a package, the triplet is always present:

```yaml
requires:
- name: "awesome"
  version: "0.1"
  category: "foo"
- name: "bar"
  version: "0.1"
  category: "foo"
```

## Building process

When a package is required to be built, Luet resolves the dependency trees and orders the spec files to satisfy the given contraints.

Each package build context is where the spec files are found (`definition.yaml` and `build.yaml`). This means that in the container, which is running the build process, the resources inside the package folder are accessible, as normally in Docker.

```
❯ tree distro/raspbian/buster
distro/raspbian/buster
├── build.sh
├── build.yaml
├── definition.yaml
└── finalize.yaml
```
In the example above, `build.sh` is accessible in build time and can be invoked easily in build time in `build.yaml`:
```yaml
steps:
- sh build.sh
```

## Package provides

Packages can specify a list of `provides`. This is a list of packages in *package form*, which indicates that the current definition *replaces* every occurrence of the packages in the list (both at *build* and *runtime*). This mechanism is particularly helpful for handling package moves or for enabling virtual packages (e.g., [gentoo virtual packages](https://packages.gentoo.org/categories/virtual)).

*Note: packages in the `provides` list don't need to exist or have a valid build definition either.*

## Package types

By a combination of keywords in `build.yaml`, you end up with categories of packages that can be built:

- Seed packages
- Packages deltas
- Package layers
- Package with includes

Check the [Specfile concept](/docs/docs/concepts/packages/specfile) page for a full overview of the available keywords in the Luet specfile format.

### Seed packages

Seed packages denote a parent package (or root) that can be used by other packages as a dependency. Normally, seed packages include just an image (preferably tagged) used as a base for other packages to depend on.

It is useful to pin to specific image versions, and to write down in a tree where packages are coming from. There can be as many seed packages as you like in a tree.

A seed package `build.yaml` example is the following:

```yaml
image: "alpine:3.1"
```

Every other package that depends on it will inherit the layers from it.

If you want to extract the content of the seed package in a separate packages (splitting), you can just create as many package as you wish depending on that one, and extract its content, for example:

**alpine/build.yaml**
```yaml
image: "alpine:3.1"
```

**alpine/definition.yaml**
```yaml
name: "alpine"
version: "3.1"
category: "seed"
```

**sh/build.yaml**
```yaml
# List of build-time dependencies
requires:
- name: "alpine"
  version: "3.1"
  category: "seed"
unpack: true # Tells luet to use the image content by unpacking it
includes: 
- /bin/sh
```

**sh/definition.yaml**
```yaml
name: "sh"
category: "utils"
version: "1.0"
```

In this example, there are two packages being specified:

- One is the `seed` package, which is the base image employed to later extract packages. It has no installable content, and it is just virtually used during build phase.
- `sh` is the package which contains `/bin/sh`, extracted from the seed image and packaged. This can be consumed by Luet clients in order to install `sh` in their system.

### Packages delta

Luet, by default, will try to calculate the delta of the package that is meant to be built. This means that it tracks **incrementally** the changes in the packages, to ease the build definition. Let's see an example.

Given the root package:
**alpine/build.yaml**
```yaml
image: "alpine:3.1"
```

**alpine/definition.yaml**
```yaml
name: "alpine"
version: "3.1"
category: "seed"
```

We can generate any file, and include it in our package by defining this simple package:

**foo/build.yaml**
```yaml
# List of build-time dependencies
requires:
- name: "alpine"
  version: "3.1"
  category: "seed"
steps:
- echo "Awesome" > /foo
```

**foo/definition.yaml**
```yaml
name: "foo"
category: "utils"
version: "1.0"
```

By analyzing the difference between the two packages, Luet will automatically track and package `/foo` as part of the `foo` package.

To allow operations that must not be accounted in to the final package, you can use the `prelude` keyword:

**foo/build.yaml**
```yaml
# List of build-time dependencies
requires:
- name: "alpine"
  version: "3.1"
  category: "seed"
prelude:
- echo "Not packaged" > /invisible
steps:
- echo "Awesome" > /foo
```

**foo/definition.yaml**
```yaml
name: "foo"
category: "utils"
version: "1.0"
```

The list of commands inside `prelude` that would produce artifacts, are not accounted to the final package. In this example, only `/foo` would be packaged (which output is equivalent to the example above).

This can be used, for instance, to fetch sources that must not be part of the package.

You can apply restrictions anytime and use the `includes` keyword to specifically pin to the files you wish in your package.

### Package layers

Luet can be used to track entire layers and make them installable by Luet clients. 

Given the examples above:

**alpine/build.yaml**
```yaml
image: "alpine:3.1"
```

**alpine/definition.yaml**
```yaml
name: "alpine"
version: "3.1"
category: "seed"
```

An installable package derived by the seed, with the actual full content of the layer can be composed as follows:

**foo/build.yaml**
```yaml
# List of build-time dependencies
requires:
- name: "alpine"
  version: "3.1"
  category: "seed"
unpack: true # It advertize Luet to consume the package as is
```

**foo/definition.yaml**
```yaml
name: "foo"
category: "utils"
version: "1.0"
```

This can be combined with other keywords to manipulate the resulting package (layer), for example:

**foo/build.yaml**
```yaml
# List of build-time dependencies
requires:
- name: "alpine"
  version: "3.1"
  category: "seed"
unpack: true # It advertize Luet to consume the package as is
steps:
- apk update
- apk add git
- apk add ..
```

**foo/definition.yaml**
```yaml
name: "foo"
category: "utils"
version: "1.0"
```

### Package includes

In addition, the `includes` keyword can be set in order to extract portions from the package image.

**git/build.yaml**
```yaml
# List of build-time dependencies
requires:
- name: "alpine"
  version: "3.1"
  category: "seed"
unpack: true # It advertize Luet to consume the package as is
steps:
- apk update
- apk add git
includes:
- /usr/bin/git
```

**foo/definition.yaml**
```yaml
name: "git"
category: "utils"
version: "1.0"
```

As a reminder, the `includes` keywords accepts regular expressions in the Golang format. Any criteria expressed by means of Golang regular expressions, and matching the file name (absolute path), will be part of the final package.
