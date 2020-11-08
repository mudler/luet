#!/bin/bash
set -ex
export LUET_NOLOCK=true

LUET_VERSION=0.8.6
LUET_ROOTFS=${LUET_ROOTFS:-/}
LUET_DATABASE_PATH=${LUET_DATABASE_PATH:-/var/luet/db}
LUET_DATABASE_ENGINE=${LUET_DATABASE_ENGINE:-boltdb}
LUET_CONFIG_PROTECT=${LUET_CONFIG_PROTECT:-1}

wget -q https://github.com/mudler/luet/releases/download/0.8.6/luet-0.8.6-linux-amd64 -O luet
chmod +x luet

mkdir -p /etc/luet/repos.conf.d || true
mkdir -p $LUET_DATABASE_PATH || true
mkdir -p /var/tmp/luet || true

if [ "${LUET_CONFIG_PROTECT}" = "1" ] ; then
  mkdir -p /etc/luet/config.protect.d || true
  wget -q  https://raw.githubusercontent.com/mudler/luet/master/contrib/config/config.protect.d/01_etc.yml.example -O /etc/luet/config.protect.d/01_etc.yml
fi
wget -q https://raw.githubusercontent.com/mocaccinoOS/repository-index/master/packages/mocaccino-repository-index/mocaccino-repository-index.yml -O /etc/luet/repos.conf.d/mocaccino-repository-index.yml

cat > /etc/luet/luet.yaml <<EOF
general:
  debug: false
system:
  rootfs: ${LUET_ROOTFS}
  database_path: "${LUET_DATABASE_PATH}"
  database_engine: "${LUET_DATABASE_ENGINE}"
  tmpdir_base: "/var/tmp/luet"
EOF

./luet install repository/luet repository/mocaccino-repository-index
./luet install system/luet system/luet-extensions

rm -rf luet
