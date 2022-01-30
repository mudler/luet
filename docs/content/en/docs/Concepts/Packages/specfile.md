---
title: "Specfile"
linkTitle: "Specfile"
weight: 2
description: >
  Luet specfile syntax
---


# Specfiles

Luet [packages](/docs/docs/concepts/packages/) are defined by specfiles. Specfiles define the runtime and builtime requirements of a package.  There is an hard distinction between runtime and buildtime. A spec is composed at least by the runtime (`definition.yaml` or a `collection.yaml`) and the buildtime specification (`build.yaml`).

Luet identifies the package definition by looking at directories that contains a `build.yaml` and a `definition.yaml` (or `collection.yaml`) files. A Luet tree is merely a composition of directories that follows this convention. There is no constriction on either folder naming or hierarchy.

*Example of a [tree folder hierarchy](https://github.com/Luet-lab/luet-embedded/tree/master/distro)*
```bash
tree distro                                                      
distro
├── funtoo              
│   ├── 1.4
│   │   ├── build.sh        
│   │   ├── build.yaml                                             
│   │   ├── definition.yaml
│   │   └── finalize.yaml
│   ├── docker
│   │   ├── build.yaml
│   │   ├── definition.yaml
│   │   └── finalize.yaml
│   └── meta
│       └── rpi
│           └── 0.1
│               ├── build.yaml
│               └── definition.yaml
├── packages
│   ├── container-diff
│   │   └── 0.15.0
│   │       ├── build.yaml
│   │       └── definition.yaml
│   └── luet
│       ├── build.yaml
│       └── definition.yaml
├── raspbian
│   ├── buster
│   │   ├── build.sh
│   │   ├── build.yaml
│   │   ├── definition.yaml
│   │   └── finalize.yaml
│   ├── buster-boot
│   │   ├── build.sh
│   │   ├── build.yaml
│   │   ├── definition.yaml
│   │   └── finalize.yaml
```

## Build specs

Build specs are defined in `build.yaml` files. They denote the build-time `dependencies` and `conflicts`, together with a definition of the content of the package.

*Example of a `build.yaml` file*:
```yaml
steps:
- echo "Luet is awesome" > /awesome
prelude:
- echo "nooops!"
requires:
- name: "echo"
  version: ">=1.0"
conflicts:
- name: "foo"
  version: ">=1.0"
provides:
- name: "bar"
  version: ">=1.0"
env:
- FOO=bar
includes:
- /awesome

unpack: true
```

### Building strategies

Luet can create packages with different strategies: 

- by delta. Luet will analyze the containers differencies to find out which files got **added**. 
  You can use the `prelude` section to exclude certains file during analysis.
- by taking a whole container content
- by considering a single directory in the build container.

#### Package by delta

By default Luet will analyze the container content and extract any file that gets **added** to it. The difference is calculated by using the container which is depending on, or either by the container which is created by running the steps in the `prelude` section of the package build spec:

```yaml
prelude:
- # do something...
steps:
- # real work that should be calculated delta over
```

By omitting the `prelude` keyword, the delta will be calculated from the parent container where the build will start from.

#### Package by container content

Luet can also generate a package content from a container. This is really useful when creating packages that are entire versioned `rootfs`. To enable this behavior, simply add `unpack: true` to the `build.yaml`. This enables the Luet unpacking features, which will extract all the files contained in the container which is built from the `prelude` and `steps` fields.

To include/exclude single files from it, use the `includes` and `excludes` directives.

#### Package by a folder in the final container

Similarly, you can tell Luet to create a package from a folder in the build container. To enable this behavior, simply add `package_dir: "/path/to/final/dir"`.
The directory must represent exactly how the files will be ultimately installed from clients, and they will show up in the same layout in the final archive.

So for example, to create a package which ships `/usr/bin/mybin`, we could write:
```yaml
package_dir: "/output"
steps:
- mkdir -p /output/usr/bin/
- echo "fancy stuff" > /output/usr/bin/mybin && chmod +x /output/usr/bin/mybin
```

### Build time dependencies

A package build spec defines how a package is built. In order to do this, Luet needs to know where to start. Hence a package must declare at least either one of the following:

- an `image` keyword which tells which Docker image to use as base, or
- a list of `requires`, which are references to other packages available in the tree.

They can't be both present in the same specfile. 

To note, it's not possible to mix package build definitions from different `image` sources. They must form a unique sub-graph in the build dependency tree. 

On the other hand it's possible to have multiple packages depending on a combination of different `requires`, given they are coming from the same `image` parent.

### Excluding/including files explictly

Luet can also *exclude* and *include* single files or folders from a package by using the `excludes` and `includes` keyword respecitvely. 

Both of them are parsed as a list of Golang regex expressions, and they can be combined together to fine-grainly decide which files should be inside the final artifact. You can refer to the files as they were in the resulting package. So if a package produces a `/foo` file,  and you want to exclude it, you can add it to `excludes` as `/foo`.

### Package source image

Luet needs an image to kick-off the build process for each package. This image is being used to run the commands in the `steps` and `prelude`, and then the image is processed by the **building strategies** explained above. 

The image can be resolved either by: 

1) providing a specific image name with `image` 
2) providing a set of package requirements with `requires` which will be constructed a new image from. The resulting image is an image linked between each other with the `FROM` field in the Dockerfile following the SAT solver ordering.
3) providing a set of packages to squash their result from `requires` and by specifying `requires_final_images: true`. 

{{< alert color="info" title="Note" >}}
The above keywords cannot be present in the same spec **at the same time**, or they cannot be combined. But you are free to create further intermediate specs to achieve the desired image.
{{< /alert >}}

#### Difference between `requires` and `requires` with `requires_final_images: true`

`requires` generates a graph from all the `images` of the specfile referenced inside the list. This means it builds a chain of images that are used to build the packages, e.g.: `packageA(image: busybox) -> packageB (requires: A) -> packageC (requires: C)`. The container which is running your build then **inherits** it's parents from a chain of order resolution, provided by the SAT solver.

When specifying `requires_final_images: true` luet builds an artifact for each of the packages listed from their compilation specs and it will later *squash* them together in a new container image which is then used in the build process to create an artifact.

The key difference is about *where* your build is going to run from. By specifying `requires_final_images` it will be constructed a new image with the content of each package - while if setting it to false, it will order the images appropriately and link them together with the Dockerfile `FROM` field. That allows to reuse the same images used to build the packages in the require section - or - create a new one from the result of each package compilation.

## Keywords

Here is a list of the full keyword refereces for the `build.yaml` file.

### `conflicts`

(optional) List of packages which it conflicts with in *build time*. In the same form of `requires` it is a list of packages that the current one is conflicting with.

```yaml
conflicts:
- name: "foo"
  category: "bar"
  version: "1.0"
...
- name: "baz"
  category: "bar"
  version: "1.0"
```

See [Package concepts](/docs/docs/concepts/packages) for more information on how to represent a package in a Luet tree.

### `copy`

_since luet>=0.15.0_

(optional) A list of packages/images where to copy files from. It is the [Docker multi-stage build](https://docs.docker.com/develop/develop-images/multistage-build/) equivalent but enhanced with tree hashing resolution.

To copy a specific file from a package *build* container:

```yaml
steps:
- ...
prelude:
- ...
copy:
- package: 
    category: "foo"
    name: "bar"
    version: ">=0"
  source: "/foo"
  destination: "/bar"
```

Any package that is listed in the section will be compiled beforeahead the package, and the file is available both in `prelude` and `steps`.

Internally, it's rendered as `COPY --from=package/image:sha /foo /bar`

To copy a specific file from an external image:

```yaml
steps:
- ...
prelude:
- ...
copy:
- image: "buxybox:latest"
  source: "/foo"
  destination: "/bar"
```

### `env`

(optional) A list of environment variables ( in `NAME=value` format ) that are expanded in `step` and in `prelude`. ( e.g. `${NAME}` ).

```yaml
env:
- PATH=$PATH:/usr/local/go/bin
- GOPATH=/luetbuild/go
- GO111MODULE=on
- CGO_ENABLED=0
- LDFLAGS="-s -w"
```

### `excludes`

(optional) List of golang regexes. They are in full path form (e.g. `^/usr/bin/foo` ) and indicates that the files listed shouldn't be part of the final artifact

Wildcards and golang regular expressions are supported. If specified, files which are not matching any of the regular expressions in the list will be excluded in the final package.

```yaml
excludes:
- ^/etc/shadow
- ^/etc/os-release
- ^/etc/gshadow
```

By combining `excludes` with `includes`, it's possible to include certain files while excluding explicitly some others (`excludes` takes precedence over `includes`).


### `image`

(optional/required) Docker image to be used to build the package.

```yaml
image: "busybox"
```

It might be omitted in place of `requires`, and indicates the image used to build the package. The image will be pulled and used to build the package.

### `includes`

(optional)  List of regular expressions to match files in the resulting package. The path is absolute as it would refer directly to the artifact content.

Wildcards and golang regular expressions are supported. If specified, files which are not matching any of the regular expressions in the list will be excluded in the final package.

```yaml
includes:
- /etc$
- /etc/lvm$
- /etc/lvm/.*
- /usr$
- /usr/bin$
- /usr/bin/cc.*
- /usr/bin/c\+\+.*
- /usr/bin/cpp.*
- /usr/bin/g\+\+.*
```

__Note__: Directories are treated as standard entries, so to include a single file, you need also to explictly include also it's directory. Consider this example to include `/etc/lvm/lvm.conf`:
```yaml
includes:
- /etc$
- /etc/lvm$
- /etc/lvm/lvm.conf
```

### `join`

_since luet>=0.16.0_
_to be deprecated in luet>=0.18.0 in favor of `requires_final_images`_

(optional/required) List of packages which are used to generate a parent image from.

It might be omitted in place of `image` or `requires`, and will generate an image which will be used as source of the package from the final packages in the above list. The new image is used to run eventually the package building process and a new artifact can be generated out of it.


```yaml
join:
- name: "foo"
  category: "bar"
  version: "1.0"
...
- name: "baz"
  category: "bar"
  version: "1.0"
```

See [Package concepts](/docs/docs/concepts/packages) for more information on how to represent a package in a Luet tree.

#### Examples

- https://github.com/mocaccinoOS/mocaccino-stage3/blob/278e3637cf65761bf01a22c891135e237e4717ad/packages/system/stage3/build.yaml

### `package_dir`

(optional) A path relative to the build container where to create the package from.

Similarly to `unpack`, changes the building strategy.


```yaml
steps:
- mkdir -p /foo/bar/etc/myapp
- touch /foo/bar/etc/myapp/config
package_dir: /foo/bar
```

### `prelude`

(optional) A list of commands to perform in the build container before building.

```yaml
prelude:
- |
   PACKAGE_VERSION=${PACKAGE_VERSION%\+*} && \
   git clone https://github.com/mudler/yip && cd yip && git checkout "${PACKAGE_VERSION}" -b build
```

### `requires`

(optional/required) List of packages which it depends on.

A list of packages that the current package depends on in *build time*. It might be omitted in place of `image`, and determines the resolution tree of the package itself. A new image is composed from the packages listed in this section in order to build the package

```yaml
requires:
- name: "foo"
  category: "bar"
  version: "1.0"
...
- name: "baz"
  category: "bar"
  version: "1.0"
```

See [Package concepts](/docs/docs/concepts/packages) for more information on how to represent a package in a Luet tree.

### `requires_final_images`

_since luet>=0.17.0_

(optional) A boolean flag which instruct luet to use the final images in the `requires` field.

By setting `requires_final_images: true` in the compilation spec, packages in the `requires` section will be first compiled, and afterwards the final packages are squashed together in a new image that will be used during build.

```yaml
requires:
- name: "foo"
  category: "bar"
  version: "1.0"
...
- name: "baz"
  category: "bar"
  version: "1.0"

requires_final_images: true
```

`requires_final_images` replaces the use of `join`, which will be deprecated in luet `>=0.18.0`.

### `step`

(optional) List of commands to perform in the build container.

```yaml
steps:
- |
   cd yip && make build-small && mv yip /usr/bin/yip
```

### `unpack`

(optional) Boolean flag. It indicates to use the unpacking strategy while building a package

```yaml
unpack: true
```

It indicates that the package content **is** the whole container content.

## Rutime specs

Runtime specification are denoted in a `definition.yaml` or a `collection.yaml` sibiling file. It identifies the package and the runtime contraints attached to it.

*definition.yaml*:
```yaml
name: "awesome"
version: "0.1"
category: "foo"
requires:
- name: "echo"
  version: ">=1.0"
  category: "bar"
conflicts:
- name: "foo"
  version: "1.0"
provides:
- name: "bar"
  version: "<1.0"
```

A `collection.yaml` can be used in place of a `definition.yaml` to identify a **set** of packages that instead shares a common `build.yaml`:

*collection.yaml*:
```yaml
packages:
- name: "awesome"
  version: "0.1"
  category: "foo"
  requires:
  - name: "echo"
    version: ">=1.0"
    category: "bar"
  conflicts:
  - name: "foo"
    version: "1.0"
  provides:
  - name: "bar"
    version: "<1.0"
- name: "awesome"
  version: "0.2"
  category: "foo"
  requires:
  - name: "echo"
    version: ">=1.0"
    category: "bar"
  conflicts:
  - name: "foo"
    version: "1.0"
  provides:
  - name: "bar"
    version: "<1.0"
...
```

All the fields (also the ones which are not part of the spec) in the `definition.yaml` file are available as templating values when rendering the `build.yaml` file. When running [finalizers](/docs/docs/concepts/packages/specfile/#finalizers) instead only the fields belonging to the specs are available.

### Keywords

Here is a list of the full keyword refereces


### `annotations`

(optional) A map of freeform package annotations:

```yaml
annotations:
  foo: "bar"
  baz: "test"
```

#### `category`

(optional) A string containing the category of the package

```yaml
category: "system"
```

### `conflicts`

(optional) List of packages which it conflicts with in *runtime*. In the same form of `requires` it is a list of packages that the current one is conflicting with.

```yaml
conflicts:
- name: "foo"
  category: "bar"
  version: "1.0"
...
- name: "baz"
  category: "bar"
  version: "1.0"
```

See [Package concepts](/docs/docs/concepts/packages) for more information on how to represent a package in a Luet tree.

### `description`

(optional) A string indicating the package description

```yaml
name: "foo"
description: "foo is capable of..."
```


### `hidden`

(optional) A boolean indicating whether the package has to be shown or not in the search results (`luet search...`)

```yaml
hidden: true
```

### `labels`

(optional) A map of freeform package labels:

```yaml
labels:
  foo: "bar"
  baz: "test"
```

Labels can be used in `luet search` to find packages by labels, e.g.:

```bash
$> luet search --by-label foo
```

### `license`

(optional) A string indicating the package license type.

```yaml
license: "GPL-3"
```

#### `name`

(required) A string containing the name of the package

```yaml
name: "foo"
```

### `provides`

(optional) List of packages which the current package is providing.

```yaml
conflicts:
- name: "foo"
  category: "bar"
  version: "1.0"
...
- name: "baz"
  category: "bar"
  version: "1.0"
```

See [Package concepts](/docs/docs/concepts/packages) for more information on how to represent a package in a Luet tree.

### `requires`

(optional) List of packages which it depends on in runtime.

A list of packages that the current package depends on in *runtime*. The determines the resolution tree of the package itself.

```yaml
requires:
- name: "foo"
  category: "bar"
  version: "1.0"
...
- name: "baz"
  category: "bar"
  version: "1.0"
```

See [Package concepts](/docs/docs/concepts/packages) for more information on how to represent a package in a Luet tree.

### `uri`

(optional) A list of URI relative to the package ( e.g. the official project pages, wikis, README, etc )

```yaml
uri:
- "http://www.mocaccino.org"
- ...
```

#### `version`

(required) A string containing the version of the package

```yaml
version: "1.0"
```




## Refering to packages from the CLI

All the `luet` commands which takes a package as argument, respect the following syntax notation:

- `cat/name`: will default to selecting any available package
- `=cat/name`: will default to gentoo parsing with regexp so also `=cat/name-1.1` works
- `cat/name@version`: will select the specific version wanted ( e.g. `cat/name@1.1` ) but can also include ranges as well `cat/name@>=1.1`
- `name`: just name, category is omitted and considered empty

## Finalizers

Finalizers are denoted in a `finalize.yaml` file, which is a sibiling of `definition.yaml` and `build.yaml` file. It contains a list of commands that finalize the package when it is installed in the machine.

*finalize.yaml*:
```yaml
install:
- rc-update add docker default
```

### Keywords

- `install`: List of commands to run in the host machine. Failures are eventually ignored, but will be reported and luet will exit non-zero in such case.
