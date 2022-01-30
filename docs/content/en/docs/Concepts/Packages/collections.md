---
title: "Collections"
linkTitle: "Collections"
weight: 4
description: >
  Group a set of package build spec with templating
---

`Collections` are a special superset of packages. To define a collection, instead of using a `definition.yaml` file, create a `collection.yaml` file with a list of packages:

{{<githubembed repo="mudler/luet" file="tests/fixtures/shared_templates/test/collection.yaml" lang="yaml">}}

Packages under a collection shares the same `build.yaml` and `finalize.yaml`, so a typical package layout can be:

```
collection/
    collection.yaml
    build.yaml
    finalize.yaml
    ... additional files in the build context
```

Luet during the build phase, will treat packages of a collection individually. A collection is a way to share the same build process across different packages.

## Templating

[The templating mechanism](/docs/docs/concepts/packages/templates) can be used in collections as well, and each stanza in `packages` is used to interpolate each single package. 

## Examples

- https://github.com/mocaccinoOS/mocaccino-musl-universe/tree/master/multi-arch/packages/entities
- https://github.com/mocaccinoOS/portage-tree/tree/master/multi-arch/packages/groups
- https://github.com/mocaccinoOS/mocaccino-musl-universe/tree/master/multi-arch/packages/X