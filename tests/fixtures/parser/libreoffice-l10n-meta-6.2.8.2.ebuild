# Copyright 2004-2008 Sabayon Linux
# Distributed under the terms of the GNU General Public License v2

EAPI=4

DESCRIPTION="LibreOffice.org localisation meta-package"
HOMEPAGE="http://www.documentfoundation.org"
LICENSE="LGPL-2"
SLOT="0"
KEYWORDS="~amd64 ~x86"
SRC_URI=""
RDEPEND=""
DEPEND=""
IUSE=""

SPELL_DIRS="af bg ca cs cy da de el en eo es et fo fr ga gl he hr hu ia id it \
lt lv mi mk nb nl pl pt ro ru sk sl sv sw tn uk zu"

LANGS="af am ar as ast be bg bn bo br brx bs ca cs cy da de dgo dz el \
en_GB en_US en_ZA eo es et eu fa fi fr ga gd gl gu gug he hi hr hu id is it ja ka kk km kn \
ko kok ks lb lo lt lv mai mk ml mn mni mr my nb ne nl nn nr nso oc om or pa_IN \
pl pt pt_BR ro ru rw sa_IN sat sd si sid sk sl sq sr ss st sv sw_TZ ta te tg \
th tn tr ts tt ug uk uz ve vi xh zh_CN zh_TW zu"

for X in ${LANGS}; do
	IUSE+=" linguas_${X}"
	RDEPEND+=" linguas_${X}? ( ~app-office/libreoffice-l10n-${X}-${PV} )"
done
for X in ${SPELL_DIRS}; do
	IUSE+=" linguas_${X}"
	RDEPEND+=" linguas_${X}? ( app-dicts/myspell-${X} )"
done
