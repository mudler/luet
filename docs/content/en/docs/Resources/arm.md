---
title: "ARM images"
linkTitle: "ARM images"
weight: 4
description: >
  Use Luet to build, track, and release OTA update for your embedded devices.
---

{{< alert color="warning" title="Warning" >}}
This article is outdated.
Please refer to the ["Hello World"](../../tutorials/hello_world/) tutorial instead.
{{< /alert >}}

Here we show an example on how to build "burnable" SD images for Raspberry Pi with Luet. This approach lets you describe and version OTA upgrades for your embedded devices, delivering upgrades as layer upgrades on the Pi.

The other good side of the medal is that you can build a Luet package repository with multiple distributions (e.g. `Raspbian`, `OpenSUSE`, `Gentoo`, ... ) and switch among them in runtime. In the above example `Raspbian` and `Funtoo` (at the time of writing) are available.

## Prerequisites

You have to run the following steps inside an ARM board to produce arm-compatible binaries. Any distribution with Docker will work. Note that the same steps could be done in a cross-compilation approach, or with qemu-binfmt in a amd64 host. 

You will also need in your host:

- Docker
- Luet installed (+container-diff) in `/usr/bin/luet` (arm build)
- make

## Build the packages

Clone the repository https://github.com/Luet-lab/luet-embedded

    $> git clone https://github.com/Luet-lab/luet-embedded
    $> cd luet-embedded
    $> sudo make build-all
    ...

If a rebuild is needed, just do `sudo make rebuild-all` after applying the changes.

## Create the repository

    $> sudo make create-repo
    ...

## Serve the repo locally

    $> make serve-repo
    ...

## Create the flashable image

### Funtoo based system

    $> sudo LUET_PACKAGES='distro/funtoo-1.4 distro/raspbian-boot-0.20191208 system/luet-develop-0.5' make image
    ...

### Raspbian based system

    $> sudo LUET_PACKAGES='distro/raspbian-0.20191208 distro/raspbian-boot-0.20191208 system/luet-develop-0.5' make image
    ...


At the end of the process, a file `luet_os.img`, ready to be flashed to an SD card, should be present in the current directory.

## Add packages

In order to build and add [packages](/docs/docs/concepts/packages/) to the exiting repository, simply add or edit the [specfiles](/docs/docs/concepts/specfile) under the `distro` folder. When doing ```make rebuild-all``` the packages will be automatically compiled and made available to the local repository.
