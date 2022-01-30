#!/bin/bash
if [ $(id -u) -ne 0 ]
  then echo "Please run the installer with sudo/as root"
  exit
fi

set -ex
export LUET_NOLOCK=true

LUET_VERSION=${LUET_VERSION:-0.22.7}
LUET_ROOTFS=${LUET_ROOTFS:-/}
LUET_DATABASE_PATH=${LUET_DATABASE_PATH:-/var/luet/db}
LUET_DATABASE_ENGINE=${LUET_DATABASE_ENGINE:-boltdb}
LUET_CONFIG_PROTECT=${LUET_CONFIG_PROTECT:-1}

if [ -z "$LUET_ARCH" ]; then
    LUET_ARCH=$(uname -m)
fi

case $LUET_ARCH in
    amd64|x86_64)
        LUET_ARCH=amd64
        ;;
    arm64|aarch64|arm*)
        LUET_ARCH=armv7
        ;;
esac

curl -L https://github.com/mudler/luet/releases/download/${LUET_VERSION}/luet-${LUET_VERSION}-linux-${LUET_ARCH} --output luet
chmod +x luet

mkdir -p /etc/luet/repos.conf.d || true
mkdir -p $LUET_DATABASE_PATH || true
mkdir -p /var/tmp/luet || true

if [ "${LUET_CONFIG_PROTECT}" = "1" ] ; then
  mkdir -p /etc/luet/config.protect.d || true
  curl -L https://raw.githubusercontent.com/mudler/luet/master/contrib/config/config.protect.d/01_etc.yml.example --output /etc/luet/config.protect.d/01_etc.yml
fi
curl -L https://raw.githubusercontent.com/mocaccinoOS/repository-index/master/packages/mocaccino-repository-index.yml --output /etc/luet/repos.conf.d/mocaccino-repository-index.yml

cat > /etc/luet/luet.yaml <<EOF
general:
  debug: false
system:
  rootfs: ${LUET_ROOTFS}
  database_path: "${LUET_DATABASE_PATH}"
  database_engine: "${LUET_DATABASE_ENGINE}"
  tmpdir_base: "/var/tmp/luet"
EOF

./luet install -y repository/luet repository/mocaccino-repository-index

case $LUET_ARCH in
    amd64|x86_64)
       ./luet install -y system/luet
       ;;
    arm64|aarch64|arm*)
       ./luet install -y system/luet-arm
       ;;
esac

if [ ! -e "/usr/bin/luet" ]; then
  cp -rf luet /usr/bin/luet
fi

rm -rf luet
