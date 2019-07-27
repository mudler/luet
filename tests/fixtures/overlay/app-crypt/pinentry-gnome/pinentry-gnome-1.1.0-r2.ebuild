# Copyright 1999-2018 Gentoo Authors
# Distributed under the terms of the GNU General Public License v2

EAPI=6

inherit autotools flag-o-matic toolchain-funcs

MY_PN=${PN/-gnome}
MY_P=${P/-gnome}
DESCRIPTION="GNOME 3 frontend for pinentry"
HOMEPAGE="https://gnupg.org/aegypten2/index.html"
SRC_URI="mirror://gnupg/${MY_PN}/${MY_P}.tar.bz2"

LICENSE="GPL-2"
SLOT="0"
KEYWORDS="~arm ~amd64 ~x86"
IUSE="caps"

CDEPEND="
	~app-crypt/pinentry-base-${PV}
	app-crypt/libsecret
	caps? ( sys-libs/libcap )
"

DEPEND="${RDEPEND}
	virtual/pkgconfig"

RDEPEND="
	${CDEPEND}
	app-crypt/gcr
	sys-libs/ncurses:0=
"

S="${WORKDIR}/${MY_P}"

PATCHES=(
	"${FILESDIR}/${MY_PN}-1.0.0-make-icon-work-under-Plasma-Wayland.patch"
	"${FILESDIR}/${MY_PN}-0.8.2-ncurses.patch"
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
		--disable-pinentry-fltk \
		--disable-pinentry-gtk2 \
		--disable-pinentry-curses \
		--enable-fallback-curses \
		--disable-pinentry-qt \
		$(use_with caps libcap) \
		--enable-libsecret \
		--enable-pinentry-gnome3
}

src_install() {
	cd gnome3 || die
	emake DESTDIR="${D}" install
}

pkg_postinst() {
	eselect pinentry set pinentry-gnome3
	# eselect pinentry update ifunset

}

pkg_postrm() {
	eselect pinentry update ifunset
}
