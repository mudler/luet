# Copyright 1999-2018 Gentoo Authors
# Distributed under the terms of the GNU General Public License v2

EAPI=6

inherit autotools flag-o-matic qmake-utils toolchain-funcs

MY_PN=${PN/-base}
MY_P=${P/-base}
DESCRIPTION="Simple passphrase entry dialogs which utilize the Assuan protocol"
HOMEPAGE="https://gnupg.org/aegypten2/index.html"
SRC_URI="mirror://gnupg/${MY_PN}/${MY_P}.tar.bz2"

LICENSE="GPL-2"
SLOT="0"
KEYWORDS="~arm ~amd64 ~x86"
IUSE="caps gtk qt5 static"

RDEPEND="
	>=dev-libs/libgpg-error-1.17
	>=dev-libs/libassuan-2.1
	>=dev-libs/libgcrypt-1.6.3

	sys-libs/ncurses:0=
	caps? ( sys-libs/libcap )
	static? ( >=sys-libs/ncurses-5.7-r5:0=[static-libs,-gpm] )
	app-eselect/eselect-pinentry
"

DEPEND="${RDEPEND}
	sys-devel/gettext
	virtual/pkgconfig
"
S=${WORKDIR}/${MY_P}

DOCS=( AUTHORS ChangeLog NEWS README THANKS TODO )

PATCHES=(
	"${FILESDIR}/${MY_PN}-1.0.0-make-icon-work-under-Plasma-Wayland.patch"
	"${FILESDIR}/${MY_PN}-0.8.2-ncurses.patch"
)

src_prepare() {
	default
	eautoreconf
}

src_configure() {
	use static && append-ldflags -static
	[[ "$(gcc-major-version)" -ge 5 ]] && append-cxxflags -std=gnu++11

	econf \
		--enable-pinentry-tty \
		--enable-pinentry-emacs \
		--disable-pinentry-fltk \
		--disable-pinentry-gtk2 \
		--enable-pinentry-curses \
		--enable-fallback-curses \
		--disable-pinentry-qt \
		$(use_with caps libcap) \
		--disable-libsecret \
		--disable-pinentry-gnome3
}

src_install() {
	default
	rm -f "${ED}"/usr/bin/pinentry || die
}

pkg_postinst() {
	if ! has_version 'app-crypt/pinentry-base'; then
		# || has_version '<app-crypt/pinentry-0.7.3'; then
		elog "We no longer install pinentry-curses and pinentry-qt SUID root by default."
		elog "Linux kernels >=2.6.9 support memory locking for unprivileged processes."
		elog "The soft resource limit for memory locking specifies the limit an"
		elog "unprivileged process may lock into memory. You can also use POSIX"
		elog "capabilities to allow pinentry to lock memory. To do so activate the caps"
		elog "USE flag and add the CAP_IPC_LOCK capability to the permitted set of"
		elog "your users."
	fi

	eselect pinentry update ifunset
	use gtk && elog "If you want pinentry for Gtk+, please install app-crypt/pinentry-gtk."
	use qt5 && elog "If you want pinentry for Qt5, please install app-crypt/pinentry-qt5."
}

pkg_postrm() {
	eselect pinentry update ifunset
}
