# Copyright 1999-2015 Gentoo Foundation
# Distributed under the terms of the GNU General Public License v2

EAPI=2

DESCRIPTION="Virtual for Linux kernel sources"
HOMEPAGE=""
SRC_URI=""

LICENSE=""
SLOT="0"
KEYWORDS="~amd64 ~arm ~hppa ~ia64 ~x86"
IUSE="firmware"

SABAYON_SOURCES="sys-kernel/sabayon-sources
		sys-kernel/server-sources
		sys-kernel/rt-sources
		sys-kernel/efikamx-sources
		sys-kernel/odroid-sources
		sys-kernel/beagle-sources
		sys-kernel/beaglebone-sources"

DEPEND="firmware? ( sys-kernel/linux-firmware )"
RDEPEND="|| (
		${SABAYON_SOURCES}
		sys-kernel/gentoo-sources
		sys-kernel/vanilla-sources
		sys-kernel/ck-sources
		sys-kernel/git-sources
		sys-kernel/hardened-sources
		sys-kernel/mips-sources
		sys-kernel/openvz-sources
		sys-kernel/pf-sources
		sys-kernel/rsbac-sources
		sys-kernel/sparc-sources
		sys-kernel/tuxonice-sources
		sys-kernel/usermode-sources
		sys-kernel/vserver-sources
		sys-kernel/xbox-sources
		sys-kernel/xen-sources
		sys-kernel/zen-sources
	)"
