# Copyright 2004-2013 Sabayon
# Distributed under the terms of the GNU General Public License v2

EAPI=5

inherit eutils systemd

DESCRIPTION="Sabayon Linux Media Center Infrastructure"
HOMEPAGE="http://www.sabayon.org/"
SRC_URI=""

RESTRICT="nomirror"
LICENSE="GPL-2"
SLOT="0"
KEYWORDS="amd64 arm x86"
IUSE=""

RDEPEND="media-tv/kodi app-misc/sabayon-live"
DEPEND=""

S="${WORKDIR}"

src_install () {
	local dir="${FILESDIR}/${PV}"

	doinitd "${dir}/init.d/sabayon-mce"
	systemd_dounit "${dir}"/systemd/*

	dodir /usr/bin
	exeinto /usr/bin
	doexe "${dir}"/bin/*

	dodir /usr/libexec
	exeinto /usr/libexec
	doexe "${dir}"/libexec/*

	dodir /usr/share/xsessions
	insinto /usr/share/xsessions
	doins "${dir}"/xsession/*.desktop
}

pkg_postinst() {
	# create new user sabayonmce
	local mygroups="users"
	local gr="lp wheel uucp audio cdrom scanner video "
	gr+="cdrw usb plugdev polkituser"

	for mygroup in ${gr}; do
		if [[ -n $(egetent group "${mygroup}") ]]; then
			mygroups+=",${mygroup}"
		fi
	done
	enewuser sabayonmce -1 /bin/sh /var/sabayonmce "${mygroups}"
}
