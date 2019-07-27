# Copyright 1999-2017 Gentoo Foundation
# Distributed under the terms of the GNU General Public License v2

EAPI=6

inherit autotools flag-o-matic toolchain-funcs

MY_PN=${PN/-gtk2}
MY_P=${P/-gtk2}
DESCRIPTION="Gtk+2 frontend for pinentry"
HOMEPAGE="https://gnupg.org/aegypten2/index.html"
SRC_URI="mirror://gnupg/${MY_PN}/${MY_P}.tar.bz2"

LICENSE="GPL-2"
SLOT="0"
KEYWORDS="~arm ~amd64 ~x86"
IUSE="caps"

RDEPEND="
	~app-crypt/pinentry-base-${PV}
	!app-crypt/pinentry-base[static]
	caps? ( sys-libs/libcap )
	x11-libs/gtk+:2
	sys-libs/ncurses:0=
"

DEPEND="${RDEPEND}
	virtual/pkgconfig"

S="${WORKDIR}/${MY_P}"

PATCHES=(
	"${FILESDIR}/${MY_PN}-0.8.2-ncurses.patch"
	"${FILESDIR}/${MY_P}-build.patch"
	"${FILESDIR}/${MY_P}-Disable-tooltips-in-keyboard-grabbing-mode.patch"
	"${FILESDIR}/${MY_P}-gtk2-Fix-a-problem-with-fvwm.patch"
)

src_prepare() {
	default
	eautoreconf
}

src_configure() {
	[[ "$(gcc-major-version)" -ge 5 ]] && append-cxxflags -std=gnu++11

	econf \
		--disable-pinentry-tty \
		--disable-pinentry-emacs \
		--enable-pinentry-gtk2 \
		--disable-pinentry-curses \
		--enable-fallback-curses \
		--disable-pinentry-qt \
		$(use_with caps libcap) \
		--disable-libsecret \
		--disable-pinentry-gnome3
}

src_install() {
	cd gtk+-2 || die
	emake DESTDIR="${D}" install
}

pkg_postinst() {
	eselect pinentry set pinentry-gtk-2
	# eselect pinentry update ifunset

}

pkg_postrm() {
	eselect pinentry update ifunset
}
