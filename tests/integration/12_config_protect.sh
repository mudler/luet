#!/bin/bash

export LUET_NOLOCK=true

oneTimeSetUp() {
export tmpdir="$(mktemp -d)"
}

oneTimeTearDown() {
    rm -rf "$tmpdir"
}

testBuild() {
    mkdir $tmpdir/testrootfs/testbuild -p
    luet build --tree "$ROOT_DIR/tests/fixtures/config_protect" \
      --destination $tmpdir/testrootfs/testbuild --compression gzip test/a
    buildst=$?
    assertEquals 'builds successfully' "$buildst" "0"
    assertTrue 'create package' "[ -e '$tmpdir/testrootfs/testbuild/a-test-1.0.package.tar.gz' ]"
}

testRepo() {
    assertTrue 'no repository' "[ ! -e '$tmpdir/testbuild/repository.yaml' ]"
    luet create-repo --tree "$ROOT_DIR/tests/fixtures/config_protect" \
    --output $tmpdir/testrootfs/testbuild \
    --packages $tmpdir/testrootfs/testbuild \
    --name "test" \
    --descr "Test Repo" \
    --urls $tmpdir/testrootfs \
    --type disk > /dev/null

    createst=$?
    assertEquals 'create repo successfully' "$createst" "0"
    assertTrue 'create repository' "[ -e '$tmpdir/testrootfs/testbuild/repository.yaml' ]"
}

testConfig() {

    mkdir $tmpdir/testrootfs/etc/luet/config.protect.d -p

    cat <<EOF > $tmpdir/testrootfs/etc/luet/config.protect.d/conf1.yml
name: "protect1"
dirs:
- /etc/
EOF

    cat <<EOF > $tmpdir/luet.yaml
general:
  debug: true
system:
  rootfs: $tmpdir/testrootfs
  database_path: "/"
  database_engine: "boltdb"
config_protect_confdir:
    - /etc/luet/config.protect.d
config_from_host: false
repositories:
   - name: "main"
     type: "disk"
     enable: true
     urls:
       - "/testbuild"
EOF
    luet config --config $tmpdir/luet.yaml
    res=$?
    assertEquals 'config test successfully' "$res" "0"
}



testInstall() {

    # Simulate previous installation
    mkdir $tmpdir/testrootfs/etc/a -p
    echo "fakeconf" > $tmpdir/testrootfs/etc/a/conf

    luet install -y --config $tmpdir/luet.yaml test/a
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"


    # Simulate config protect
    assertTrue 'package A installed' "[ -e '$tmpdir/testrootfs/c' ]"
    assertTrue 'config protect created' "[ -e '$tmpdir/testrootfs/etc/a/._cfg0001_conf' ]"
}


testUnInstall() {
    luet uninstall -y --full --config $tmpdir/luet.yaml test/a
    installst=$?
    assertEquals 'uninstall test successfully' "$installst" "0"
    assertTrue 'package uninstalled' "[ ! -e '$tmpdir/testrootfs/c' ]"
    # TODO: we need remove it or not??
    assertTrue 'config protect created' "[ -e '$tmpdir/testrootfs/etc/a/._cfg0001_conf' ]"
    assertTrue 'config protect maintains the protected files' "[ -e '$tmpdir/testrootfs/etc/a/conf' ]"
}


testCleanup() {
    luet cleanup --config $tmpdir/luet.yaml
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    assertTrue 'package installed' "[ ! -e '$tmpdir/testrootfs/packages/a-test-1.0.package.tar.gz' ]"
}

# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2

