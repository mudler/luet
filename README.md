# luet - Container-based Package manager
[![Go Report Card](https://goreportcard.com/badge/github.com/mudler/luet)](https://goreportcard.com/report/github.com/mudler/luet)
[![Build Status](https://travis-ci.org/mudler/luet.svg?branch=master)](https://travis-ci.org/mudler/luet)
[![GoDoc](https://godoc.org/github.com/mudler/luet?status.svg)](https://godoc.org/github.com/mudler/luet)
[![codecov](https://codecov.io/gh/mudler/luet/branch/master/graph/badge.svg)](https://codecov.io/gh/mudler/luet)

Luet is a Package Manager based off from containers - it uses Docker (and other tech) to sandbox your builds and generate packages from them. It has no dependencies and it is well suitable for "from scratch" environments.

## In a glance

- Luet can reuse Gentoo's portage tree hierarchy, and it is heavily inspired from it.
- It builds, installs, uninstalls and perform upgrades on machines
- Installer doesn't depend on anything
- Support for packages as "layers"
- It uses SAT solving techniques to solve the deptree ( Inspired by [OPIUM](https://ranjitjhala.github.io/static/opium.pdf) )

## Status

Luet is not feature-complete yet, it can build, install/uninstall/upgrade packages - but it doesn't support yet all the features you would normally expect from a Package Manager nowadays.

Check out the [Wiki](https://github.com/mudler/luet/wiki) for more informations.
