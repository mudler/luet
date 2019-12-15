# Copyright 1999-2019 Gentoo Authors
# Distributed under the terms of the GNU General Public License v2

EAPI=6

WANT_AUTOMAKE="none"
MY_P="${P/_/-}"

MY_SVN_PN="subversion"
MY_SVN_P="${MY_SVN_PN}-${PV}"
MY_SVN_PF="${MY_SVN_PN}-${PVR}"

inherit autotools db-use depend.apache flag-o-matic libtool multilib xdg-utils

DESCRIPTION="Subversion WebDAV support"
HOMEPAGE="https://subversion.apache.org/"
SRC_URI="mirror://apache/${MY_SVN_PN}/${MY_SVN_P}.tar.bz2
	https://dev.gentoo.org/~polynomial-c/${MY_SVN_PN}-1.10.0_rc1-patches-1.tar.xz"
S="${WORKDIR}/${MY_SVN_P}"

LICENSE="Subversion"
SLOT="0"
KEYWORDS="~amd64 ~arm ~x86"
IUSE="berkdb debug +dso nls sasl"
# w/o: ctypes-python doc extras gnome-keyring java kde perl python ruby
# test vim-syntax; implicit: apache2, http

# This is an ebuild that provides mod_dav_svn and friends (it does more or
# less the same as when USE="apache2 http" is added to dev-vcs/subversion - basically
# provides three Apache modules and a configuration file), suitable for binary
# packages.
# Some flags in IUSE and their handling are only used to enforce the code be
# compiled sufficiently the same way Subversion itself was - extra carefulness.
# In the process of building libraries for WebDAV, a few unused libraries are
# also built (not the whole Subversion, though, which is a win). Some build
# time dependencies here are just for them.

# If you are building it for yourself, you don't need it.
# USE=apache2 emerge dev-vcs/subversion will do what you want.
# However, you can use this ebuild too.

# variable specific to www-apache/mod_dav_svn
MY_CDEPS="
	~dev-vcs/subversion-${PV}[berkdb=,debug=,dso=,nls=,sasl=,http]
	app-arch/bzip2
	app-arch/lz4
	>=dev-db/sqlite-3.7.12
	>=dev-libs/apr-1.3:1
	>=dev-libs/apr-util-1.3:1
	dev-libs/expat
	dev-libs/libutf8proc:=
	sys-apps/file
	sys-libs/zlib
	berkdb? ( >=sys-libs/db-4.0.14:= )
"

DEPEND="${MY_CDEPS}
	>=net-libs/serf-1.3.4
	sasl? ( dev-libs/cyrus-sasl )
	virtual/pkgconfig
	!!<sys-apps/sandbox-1.6
	nls? ( sys-devel/gettext )
	sys-apps/file"
RDEPEND="${MY_CDEPS}
	!dev-vcs/subversion[apache2]

	www-servers/apache[apache2_modules_dav]
	nls? ( virtual/libintl )"

need_apache # was: want_apache

pkg_setup() {
	if use berkdb ; then
		local apu_bdb_version="$(${EPREFIX}/usr/bin/apu-1-config --includes \
			| grep -Eoe '-I${EPREFIX}/usr/include/db[[:digit:]]\.[[:digit:]]' \
			| sed 's:.*b::')"
		einfo
		if [[ -z "${SVN_BDB_VERSION}" ]] ; then
			if [[ -n "${apu_bdb_version}" ]] ; then
				SVN_BDB_VERSION="${apu_bdb_version}"
				einfo "Matching db version to apr-util"
			else
				SVN_BDB_VERSION="$(db_ver_to_slot "$(db_findver sys-libs/db 2>/dev/null)")"
				einfo "SVN_BDB_VERSION variable isn't set. You can set it to enforce using of specific version of Berkeley DB."
			fi
		fi
		einfo "Using: Berkeley DB ${SVN_BDB_VERSION}"
		einfo

		if [[ -n "${apu_bdb_version}" && "${SVN_BDB_VERSION}" != "${apu_bdb_version}" ]]; then
			eerror "APR-Util is linked against Berkeley DB ${apu_bdb_version}, but you are trying"
			eerror "to build Subversion with support for Berkeley DB ${SVN_BDB_VERSION}."
			eerror "Rebuild dev-libs/apr-util or set SVN_BDB_VERSION=\"${apu_bdb_version}\"."
			eerror "Aborting to avoid possible run-time crashes."
			die "Berkeley DB version mismatch"
		fi
	fi

	# depend.apache_pkg_setup

	# https://issues.apache.org/jira/browse/SVN-4813#comment-16813739
	append-cppflags -P

	if use debug ; then
		append-cppflags -DSVN_DEBUG -DAP_DEBUG
	fi

	# http://mail-archives.apache.org/mod_mbox/subversion-dev/201306.mbox/%3C51C42014.3060700@wandisco.com%3E
	[[ ${CHOST} == *-solaris2* ]] && append-cppflags -D__EXTENSIONS__

	# Allow for custom repository locations.
	SVN_REPOS_LOC="${SVN_REPOS_LOC:-${EPREFIX}/var/svn}"
}

src_prepare() {
	eapply "${WORKDIR}/patches"
	eapply_user

	chmod +x build/transform_libtool_scripts.sh || die

	sed -i \
		-e "s/\(BUILD_RULES=.*\) bdb-test\(.*\)/\1\2/g" \
		-e "s/\(BUILD_RULES=.*\) test\(.*\)/\1\2/g" configure.ac

	# this bites us in particular on Solaris
	sed -i -e '1c\#!/usr/bin/env sh' build/transform_libtool_scripts.sh || \
		die "/bin/sh is not POSIX shell!"

	eautoconf
	elibtoolize

	#sed -e 's/\(libsvn_swig_py\)-\(1\.la\)/\1-$(EPYTHON)-\2/g' \
	#-i build-outputs.mk || die "sed failed"

	xdg_environment_reset
}

src_configure() {
	local myconf=(
		--libdir="${EPREFIX%/}/usr/$(get_libdir)"
		--with-apache-libexecdir
		--with-apxs="${EPREFIX}"/usr/bin/apxs
		$(use_with berkdb berkeley-db "db.h:${EPREFIX%/}/usr/include/db${SVN_BDB_VERSION}::db-${SVN_BDB_VERSION}")
		--without-ctypesgen
		$(use_enable dso runtime-module-search)
		--without-gnome-keyring
		--disable-javahl
		--without-jdk
		--without-kwallet
		$(use_enable nls)
		$(use_with sasl)
		--with-serf
		--with-apr="${EPREFIX%/}/usr/bin/apr-1-config"
		--with-apr-util="${EPREFIX%/}/usr/bin/apu-1-config"
		--disable-experimental-libtool
		--without-jikes
		--disable-mod-activation
		--disable-static
		--enable-svnxx
	)

	#use python || use perl || use ruby
	myconf+=( --without-swig )

	#use java
	myconf+=( --without-junit )

	case ${CHOST} in
		*-aix*)
			# avoid recording immediate path to sharedlibs into executables
			append-ldflags -Wl,-bnoipath
		;;
		*-cygwin*)
			# no LD_PRELOAD support, no undefined symbols
			myconf+=( --disable-local-library-preloading LT_LDFLAGS=-no-undefined )
			;;
		*-interix*)
			# loader crashes on the LD_PRELOADs...
			myconf+=( --disable-local-library-preloading )
		;;
		*-solaris*)
			# need -lintl to link
			use nls && append-libs intl
			# this breaks installation, on x64 echo replacement is 32-bits
			myconf+=( --disable-local-library-preloading )
		;;
		*-mint*)
			myconf+=( --enable-all-static --disable-local-library-preloading )
		;;
		*)
			# inject LD_PRELOAD entries for easy in-tree development
			myconf+=( --enable-local-library-preloading )
		;;
	esac

	#workaround for bug 387057
	has_version =dev-vcs/subversion-1.6* && myconf+=( --disable-disallowing-of-undefined-references )

	#version 1.7.7 again tries to link against the older installed version and fails, when trying to
	#compile for x86 on amd64, so workaround this issue again
	#check newer versions, if this is still/again needed
	#myconf+=( --disable-disallowing-of-undefined-references )

	# allow overriding Python include directory
	#ac_cv_path_RUBY=$(usex ruby "${EPREFIX%/}/usr/bin/ruby${RB_VER}" "none")
	#ac_cv_path_RDOC=(usex ruby "${EPREFIX%/}/usr/bin/rdoc${RB_VER}" "none")
	ac_cv_python_includes='-I$(PYTHON_INCLUDEDIR)' \
	econf "${myconf[@]}"
}

src_compile() {
	emake apache-mod
}

src_install() {
	emake DESTDIR="${D}" install-mods-shared

	# Install Apache module configuration.
	#use apache2
	keepdir "${APACHE_MODULES_CONFDIR}"
	insinto "${APACHE_MODULES_CONFDIR}"
	doins "${FILESDIR}/47_mod_dav_svn.conf"
}

pkg_preinst() {
	# Compare versions of Berkeley DB, bug 122877.
	if use berkdb && [[ -f "${EROOT}/usr/bin/svn" ]] ; then
		OLD_BDB_VERSION="$(scanelf -nq "${EROOT}/usr/$(get_libdir)/libsvn_subr-1$(get_libname 0)" | grep -Eo "libdb-[[:digit:]]+\.[[:digit:]]+" | sed -e "s/libdb-\(.*\)/\1/")"
		NEW_BDB_VERSION="$(scanelf -nq "${ED%/}/usr/$(get_libdir)/libsvn_subr-1$(get_libname 0)" | grep -Eo "libdb-[[:digit:]]+\.[[:digit:]]+" | sed -e "s/libdb-\(.*\)/\1/")"
		if [[ "${OLD_BDB_VERSION}" != "${NEW_BDB_VERSION}" ]] ; then
			CHANGED_BDB_VERSION="1"
		fi
	fi
}

pkg_postinst() {
	if [[ -n "${CHANGED_BDB_VERSION}" ]] ; then
		ewarn "You upgraded from an older version of Berkeley DB and may experience"
		ewarn "problems with your repository. Run the following commands as root to fix it:"
		ewarn "    db4_recover -h ${SVN_REPOS_LOC}/repos"
		ewarn "    chown -Rf apache:apache ${SVN_REPOS_LOC}/repos"
	fi

	#ewarn "If you run subversion as a daemon, you will need to restart it to avoid module mismatches."

	# from src_install in Gentoo ebuild:
	##adjust default user and group with disabled apache2 USE flag, bug 381385
	#use apache2 || sed -e "s\USER:-apache\USER:-svn\g" \
	#		-e "s\GROUP:-apache\GROUP:-svnusers\g" \
	#		-i "${ED}"etc/init.d/svnserve || die
	#use apache2 || sed -e "0,/apache/s//svn/" \
	#		-e "s:apache:svnusers:" \
	#		-i "${ED}"etc/xinetd.d/svnserve || die
	# We need to address it here with a message (when Subversion ebuild is
	# intented to be build with USE=-apache2).
	# Also, user doesn't need to tweak init.d script - user and group can
	# be changed in conf.d.
	elog "svnserve users: You may want to change user and group in /etc/conf.d/svnserve"
	elog "and /etc/xinetd.d/svnserve from current svn:svnusers to apache:apache,"
	elog "especially if you want to make use of emerge --config	${CATEGORY}/${PN}"
	elog "and its default ownership settings."
}

#pkg_postrm()

pkg_config() {
	# Remember: Don't use ${EROOT}${SVN_REPOS_LOC} since ${SVN_REPOS_LOC}
	# already has EPREFIX in it
	einfo "Initializing the database in ${SVN_REPOS_LOC}..."
	if [[ -e "${SVN_REPOS_LOC}/repos" ]] ; then
		echo "A Subversion repository already exists and I will not overwrite it."
		echo "Delete \"${SVN_REPOS_LOC}/repos\" first if you're sure you want to have a clean version."
	else
		mkdir -p "${SVN_REPOS_LOC}/conf"

		einfo "Populating repository directory..."
		# Create initial repository.
		"${EROOT}/usr/bin/svnadmin" create "${SVN_REPOS_LOC}/repos"

		einfo "Setting repository permissions..."
		SVNSERVE_USER="$(. "${EROOT}/etc/conf.d/svnserve"; echo "${SVNSERVE_USER}")"
		SVNSERVE_GROUP="$(. "${EROOT}/etc/conf.d/svnserve"; echo "${SVNSERVE_GROUP}")"
		#use apache2
		[[ -z "${SVNSERVE_USER}" ]] && SVNSERVE_USER="apache"
		[[ -z "${SVNSERVE_GROUP}" ]] && SVNSERVE_GROUP="apache"
		#use !apache2
		#[[ -z "${SVNSERVE_USER}" ]] && SVNSERVE_USER="svn"
		#[[ -z "${SVNSERVE_GROUP}" ]] && SVNSERVE_GROUP="svnusers"

		chmod -Rf go-rwx "${SVN_REPOS_LOC}/conf"
		chmod -Rf o-rwx "${SVN_REPOS_LOC}/repos"
		echo "Please create \"${SVNSERVE_GROUP}\" group if it does not exist yet."
		echo "Afterwards please create \"${SVNSERVE_USER}\" user with homedir \"${SVN_REPOS_LOC}\""
		echo "and as part of the \"${SVNSERVE_GROUP}\" group if it does not exist yet."
		echo "Finally, execute \"chown -Rf ${SVNSERVE_USER}:${SVNSERVE_GROUP} ${SVN_REPOS_LOC}/repos\""
		echo "to finish the configuration."
	fi
}
