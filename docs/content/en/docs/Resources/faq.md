---
title: "Frequently Asked Questions"
linkTitle: "FAQ"
weight: 4
description: >
  FAQ
---

## Can't build packages

There might be several reasons why packages fails to build, for example, if your build fails like this:

```
$ luet build ...

 INFO   Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?
  ERROR    Error: Failed compiling development/toolchain-go-0.6: failed building package image: Could not push image: quay.io/mocaccino/micro-toolchain:latest toolchain-go-development-0.6-builder.dockerfile: Could not build image: quay.io/mocaccino/micro-toolchain:latest toolchain-go-development-0.6-builder.dockerfile: Failed running command: : exit status 1
  ERROR    Bailing out
```

means the user you are running the build command can't either connect to docker or `docker` is not started.

Check if the user you are running the build is in the `docker` group, or if the `docker` daemon is started.

Luet by default if run with multiple packages summarize errors and can be difficult to navigate to logs, but if you think you might have found a bug, run the build with `--debug` before opening an issue.

## Why the name `luet`?

Well, I have the idea that programs should be small, so they are not difficult to type and easy to remember, and easy to stick in. `luet` is really a combination of the first letters of my fiancee name (Lucia) and my name (Ettore) `lu+et = luet`! and besides, happen to be also a [small bridge](http://www.comuniterrae.it/punto/ponte-luet/) in Italy ;)

