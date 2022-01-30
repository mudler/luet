---
title: "Templated packages"
linkTitle: "Templated packages"
weight: 3
description: >
  Use templates to fine tune build specs
---

Luet supports the [`sprig` rendering engine template, like helm](http://masterminds.github.io/sprig/). It's being used to interpolate `build.yaml` and `finalize.yaml` files before their execution. The following document assumes you are familiar with the `helm` templating.

The `build.yaml` and `finalize.yaml` files are rendered during build time, and it's possible to use the `helm` templating syntax inside such files. The `definition.yaml` file will be used to interpolate templating values available in `build.yaml`

Given the following `definition.yaml`:

```yaml
name: "test"
category: "foo"
version: "1.1"

additional_field: "baz"
```

A `build.yaml` can look like the following, and interpolates it's values during build time:

```yaml
image: ...

steps:
- echo {{.Values.name}} > /package_name
- echo {{.Values.additional_field}} > /extra

```

Which would be for example automatically rendered by luet like the following:

```yaml

image: ...

steps:
- echo test > /package_name
- echo baz > /extra

```

This mechanism can be used in collections as well, and each stanza in `packages` is used to interpolate each single package.

## Interpolating globally

It's possible to interpolate during build phase all the package specs targeted for build with the ```--values``` flag, which takes a yaml file of an arbitrary format, if variables are clashing, the yaml supplied in `--values` takes precedence and overwrite the values of each single `definition.yaml` file.

## Shared templates

Since luet `0.17.5` it is possible to share templates across different packages. All templates blocks found inside the `templates` folder inside the root `luet tree` of a repository gets templated and shared across all the packages while rendering each compilation spec of the given tree.

Consider the following:

```
shared_templates
├── templates
│   └── writefile.yaml
└── test
    ├── build.yaml
    └── collection.yaml
```
#### `collection.yaml`
We define here two packages with a collection. They will share the same compilation spec to generate two different packages
{{<githubembed repo="mudler/luet" file="tests/fixtures/shared_templates/test/collection.yaml" lang="yaml">}}


#### `writefile.yaml`

All the files in the `templates` folder will get rendered by the template for each package in the tree. We define here a simple block to write out a file from the context which is passed by:
{{<githubembed repo="mudler/luet" file="tests/fixtures/shared_templates/templates/writefile.yaml" lang="yaml">}}


#### `build.yaml`
Finally the build spec consumes the template block we declared above, passing by the name of the package:
{{<githubembed repo="mudler/luet" file="tests/fixtures/shared_templates/test/build.yaml" lang="yaml">}}


## Limitations

The `finalize.yaml` file has access only to the package fields during templating. Extra fields that are present in the `definition.yaml` file are *not* accessible during rendering in the `finalize.yaml` file, but only the package fields (`name`, `version`, `labels`, `annotations`, ...)

## References

- [Sprig docs](http://masterminds.github.io/sprig/)
- [Helm Templating functions](https://helm.sh/docs/chart_template_guide/function_list/)
- [Helm Templating variable](https://helm.sh/docs/chart_template_guide/variables/)

## Examples

- https://github.com/mocaccinoOS/mocaccino-musl-universe/tree/master/multi-arch/packages/tar

