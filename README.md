
<p align="center">
  <img width=150 height=150 src="https://user-images.githubusercontent.com/2420543/119691600-0293d700-be4b-11eb-827f-49ff1174a07a.png">
</p>

# luet - Container-based Package manager

[![Docker Repository on Quay](https://quay.io/repository/luet/base/status "Docker Repository on Quay")](https://quay.io/repository/luet/base)
[![Build and release on push](https://github.com/mudler/luet/actions/workflows/release.yml/badge.svg)](https://github.com/mudler/luet/actions/workflows/release.yml)
[![GoDoc](https://godoc.org/github.com/mudler/luet?status.svg)](https://godoc.org/github.com/mudler/luet)
[![codecov](https://codecov.io/gh/mudler/luet/branch/master/graph/badge.svg)](https://codecov.io/gh/mudler/luet)

Luet is a multi-platform Package Manager based off from containers - it uses Docker (and others) to build packages. It has zero dependencies and it is well suitable for "from scratch" environments. It can also version entire rootfs and enables delivery of OTA-alike updates, making it a perfect fit for the Edge computing era and IoT embedded devices.

It offers a simple [specfile format](https://luet-lab.github.io/docs/docs/concepts/packages/specfile/) in YAML notation to define both [packages](https://luet-lab.github.io/docs/docs/concepts/packages/) and [rootfs](https://luet-lab.github.io/docs/docs/concepts/packages/#package-layers). As it is based on containers, it can be also used to build stages for Linux From Scratch installations and it can build and track updates for those systems.

It is written entirely in Golang and where used as package manager, it can run in from scratch environment, with zero dependencies.

[![asciicast](https://asciinema.org/a/388348.svg)](https://asciinema.org/a/388348)


## In a glance

- Luet can reuse Gentoo's portage tree hierarchy, and it is heavily inspired from it.
- It builds from containers, but installs, uninstalls and perform upgrades on machines
- Installer doesn't depend on anything ( 0 dep installer !), statically built
- You can install it aside also with your current distro package manager, and start building and distributing your packages
- [Support for packages as "layers"](https://luet-lab.github.io/docs/docs/concepts/packages/specfile/#building-strategies)
- [It uses SAT solving techniques to solve the deptree](https://luet-lab.github.io/docs/docs/concepts/overview/constraints/) ( Inspired by [OPIUM](https://ranjitjhala.github.io/static/opium.pdf) )
- Support for [collections](https://luet-lab.github.io/docs/docs/concepts/packages/collections/) and [templated package definitions](https://luet-lab.github.io/docs/docs/concepts/packages/templates/)
- [Can be extended with Plugins and Extensions](https://luet-lab.github.io/docs/docs/concepts/plugins-and-extensions/)
- [Can build packages in Kubernetes (experimental)](https://github.com/mudler/luet-k8s)
- Uses containerd/go-containerregistry to manipulate images - works also daemonless with the img backend

## Install

To install luet, you can grab a release on the [Release page](https://github.com/mudler/luet/releases) or to install it in your system:

```bash
$ curl https://get.mocaccino.org/luet/get_luet_root.sh | sudo sh
$ luet search ...
$ luet install ..
$ luet --help
```

## Build from source

```bash
$ git clone https://github.com/mudler/luet.git
$ cd luet
$ make build
```

## Documentation

[Documentation](https://luet-lab.github.io/docs) is available, or
run `luet --help`,  any subcommand is documented as well, try e.g.: `luet build --help`.

# Dependency solving

Luet uses SAT and Reinforcement learning engine for dependency solving.
It encodes the package requirements into a SAT problem, using [gophersat](https://github.com/crillab/gophersat) to solve the dependency tree and give a concrete model as result.

## SAT encoding

Each package and its constraints are encoded and built around [OPIUM](https://ranjitjhala.github.io/static/opium.pdf). Additionally, Luet treats
also selectors seamlessly while building the model, adding *ALO* ( *At least one* ) and *AMO* ( *At most one* ) rules to guarantee coherence within the installed system.

## Reinforcement learning

Luet also implements a small and portable qlearning agent that will try to solve conflict on your behalf
when they arises while trying to validate your queries against the system model.

To leverage it, simply pass ```--solver-type qlearning``` to the subcommands that supports it ( you can check out by invoking ```--help``` ).


## Authors

Luet is here thanks to our amazing [contributors](https://github.com/mudler/luet/graphs/contributors)!.

Luet was originally created by Ettore Di Giacinto, mudler@sabayon.org, mudler@gentoo.org.

## License

Luet is distributed under the terms of GPLv3, check out the LICENSE file.
