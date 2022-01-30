---
title: "Overview"
linkTitle: "Overview"
weight: 1
description: >
  See Luet in action.
---


Luet provides an abstraction layer on top of the container image layer to make the package a first class construct. A package definition and all its dependencies are translated by Luet to Dockerfiles which can then be built anywhere that docker runs.

Luet is written entirely in Go and comes as a single static binary. This has a few advantages:

- Easy to recover. You can use luet to bootstrap the system entirely from the ground-up.
- Package manager has no dependencies on the packages that it installs. There is no chance of breaking the package manager by installing a conflicting package, or uninstalling one.
- Portable - it can run on any architecture

Luet brings the containers ecosystem to standard software package management and delivery. It is fully built around the container concept, and leverages the huge catalog already present in the wild. It lets you use Docker images from [Docker Hub](https://hub.docker.com/), or from private registries to build packages, and helps you to redistribute them.

Systems that are using luet as a package manager can consume Luet repositories with only luet itself. No dependency is required by the Package manager, giving you the full control on what you install or not in the system. It can be used to generate *Linux from Scratch* distributions,  also to build Docker images, or to simply build standalone packages that you might want to redistribute.

The syntax proposed aims to be [KISS](https://en.wikipedia.org/wiki/KISS_principle) - you define a set of steps that need to be run to build your image, and a set of constraints denoting the requirements or conflicts of your package.

## Why another Package manager?

There is no known package manager with 0-dependency that fully leverages the container ecosystem. This gap forces current package managers to depend on a specific system layout as base of the building process and the corresponding depencies. This can cause situations leading to a broken system. We want to fix that by empowering the user, by building their own packages, and redistribute them. 
Luet allows also to create packages entirely from Docker images content. In this way the user can actually bundle all the files of an image into a package and deliver part of it, or entirely as a layer. All of that, without the package manager depending on a single bit from it.

## Package definitions

Luet uses [YAML](https://en.wikipedia.org/wiki/YAML) for the package specification format, Luet parses the [requirements](/docs/docs/concepts/overview/constraints) to build [packages](/docs/docs/concepts/packages), so Luet can consume them.

Below you can find links to tutorials on how to build packages, images and repositories.
