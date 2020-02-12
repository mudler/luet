# luet - Container-based Package manager
[![Go Report Card](https://goreportcard.com/badge/github.com/mudler/luet)](https://goreportcard.com/report/github.com/mudler/luet)
[![Build Status](https://travis-ci.org/mudler/luet.svg?branch=master)](https://travis-ci.org/mudler/luet)
[![GoDoc](https://godoc.org/github.com/mudler/luet?status.svg)](https://godoc.org/github.com/mudler/luet)
[![codecov](https://codecov.io/gh/mudler/luet/branch/master/graph/badge.svg)](https://codecov.io/gh/mudler/luet)

Luet is a multi-platform Package Manager based off from containers - it uses Docker (and other tech) to sandbox your builds and generate packages from them. It has zero dependencies and it is well suitable for "from scratch" environments. It can also version entire rootfs and enables delivery of OTA-alike updates, making it a perfect fit for the Edge computing era and IoT embedded devices.

It offers a simple [specfile format](https://luet-lab.github.io/docs/docs/concepts/specfile/) in YAML notation to define both packages and rootfs. As it is based on containers, it can be used to build seed stages for Linux From Scratch installations and it can build and track updates for those systems.

It is written entirely in Golang and where used as package manager, it can run in from scratch environment, with zero dependencies.

## In a glance

- Luet can reuse Gentoo's portage tree hierarchy, and it is heavily inspired from it.
- It builds, installs, uninstalls and perform upgrades on machines
- Installer doesn't depend on anything ( 0 dep installer !), statically built
- Support for packages as "layers"
- It uses SAT solving techniques to solve the deptree ( Inspired by [OPIUM](https://ranjitjhala.github.io/static/opium.pdf) )

## Install

To install luet, you can grab a release on the [Release page](https://github.com/mudler/luet/releases) or compile it in your machine (requires Golang installed):

    $ git clone https://github.com/mudler/luet.git
    $ cd luet
    $ make build

## Status

Luet is not feature-complete yet, it can build, install/uninstall/upgrade packages - but it doesn't support yet all the features you would normally expect from a Package Manager nowadays.

# Dependency solving

Luet uses SAT and Reinforcement learning engine for dependency solving.
It encodes the package requirements into a SAT problem, using gophersat to solve the dependency tree and give a concrete model as result.

## SAT encoding

Each package and its constraints are encoded and built around [OPIUM](https://ranjitjhala.github.io/static/opium.pdf). Additionally, Luet treats
also selectors seamlessly while building the model, adding *ALO* ( *At least one* ) and *AMO* ( *At most one* ) rules to guarantee coherence within the installed system.

## Reinforcement learning

Luet also implements a small and portable qlearning agent that will try to solve conflict on your behalf
when they arises while trying to validate your queries against the system model.

To leverage it, simply pass ```--solver-type qlearning``` to the subcommands that supports it ( you can check out by invoking ```--help``` ).

## Documentation

[Documentation](https://luet-lab.github.io/docs) is available, or
run `luet --help`,  any subcommand is documented as well, try e.g.: `luet build --help`.

## Authors

Luet is here thanks to our amazing [contributors](https://github.com/mudler/luet/graphs/contributors)!.

Luet was originally created by Ettore Di Giacinto, mudler@sabayon.org, mudler@gentoo.org.

## License

Luet is distributed under the terms of GPLv3, check out the LICENSE file.
