# Copyright 1999-2018 Gentoo Authors
# Distributed under the terms of the GNU General Public License v2

EAPI=6

inherit autotools flag-o-matic qmake-utils toolchain-funcs

MY_PN=${PN/-qt5}
MY_P=${P/-qt5}
DESCRIPTION="Qt5 frontend for pinentry"
HOMEPAGE="https://gnupg.org/aegypten2/index.html"
SRC_URI="mirror://gnupg/${MY_PN}/${MY_P}.tar.bz2"

LICENSE="GPL-2"
SLOT="0"
KEYWORDS="~arm ~amd64 ~x86"
IUSE="caps"

RDEPEND="
	~app-crypt/pinentry-base-${PV}
	!app-crypt/pinentry-base[static]
	!app-crypt/pinentry-qt4
	caps? ( sys-libs/libcap )
	dev-qt/qtcore:5
	dev-qt/qtgui:5
	dev-qt/qtwidgets:5
	sys-libs/ncurses:0=
"
DEPEND="${RDEPEND}
	virtual/pkgconfig
"

S=${WORKDIR}/${MY_P}

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

	export QTLIB="$(qt5_get_libdir)"

	econf \
		--disable-pinentry-tty \
		--disable-pinentry-emacs \
		--disable-pinentry-fltk \
		--disable-pinentry-gtk2 \
		--disable-pinentry-curses \
		--enable-fallback-curses \
		$(use_with caps libcap) \
		--disable-libsecret \
		--disable-pinentry-gnome3 \
		--enable-pinentry-qt \
		MOC="$(qt5_get_bindir)"/moc
}

src_install() {
	cd qt || die
	emake DESTDIR="${D}" install

	dosym pinentry-qt /usr/bin/pinentry-qt4
}

pkg_postinst() {
	# -qt4 is not a typo: see dosym above.
	eselect pinentry set pinentry-qt4
	# eselect pinentry update ifunset
}

pkg_postrm() {
	eselect pinentry update ifunset
}
