#!/bin/bash

export LUET_NOLOCK=true

oneTimeSetUp() {
export tmpdir="$(mktemp -d)"
}

oneTimeTearDown() {
    rm -rf "$tmpdir"
}

testBuild() {
    mkdir $tmpdir/testbuild
    [ "$LUET_BACKEND" == "img" ] && startSkipping
    luet build --tree "$ROOT_DIR/tests/fixtures/retrieve-integration" --destination $tmpdir/testbuild --compression gzip test/b
    buildst=$?
    assertEquals 'builds successfully' "$buildst" "0"
    assertTrue 'create package dep B' "[ -e '$tmpdir/testbuild/b-test-1.0.package.tar.gz' ]"
    assertTrue 'create package' "[ -e '$tmpdir/testbuild/a-test-1.0.package.tar.gz' ]"
}

testRepo() {
    [ "$LUET_BACKEND" == "img" ] && startSkipping
    assertTrue 'no repository' "[ ! -e '$tmpdir/testbuild/repository.yaml' ]"
    luet create-repo --tree "$ROOT_DIR/tests/fixtures/retrieve-integration" \
    --output $tmpdir/testbuild \
    --packages $tmpdir/testbuild \
    --name "test" \
    --descr "Test Repo" \
    --urls $tmpdir/testrootfs \
    --type disk > /dev/null

    createst=$?
    assertEquals 'create repo successfully' "$createst" "0"
    assertTrue 'create repository' "[ -e '$tmpdir/testbuild/repository.yaml' ]"
}

testConfig() {
    mkdir $tmpdir/testrootfs
    cat <<EOF > $tmpdir/luet.yaml
general:
  debug: true
system:
  rootfs: $tmpdir/testrootfs
  database_path: "/"
  database_engine: "boltdb"
config_from_host: true
repositories:
   - name: "main"
     type: "disk"
     enable: true
     urls:
       - "$tmpdir/testbuild"
EOF
    luet config --config $tmpdir/luet.yaml
    res=$?
    assertEquals 'config test successfully' "$res" "0"
}



testInstall() {
    [ "$LUET_BACKEND" == "img" ] && startSkipping
    luet install -y --config $tmpdir/luet.yaml test/b
    #luet install -y --config $tmpdir/luet.yaml test/c-1.0 > /dev/null
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    assertTrue 'package B installed' "[ -e '$tmpdir/testrootfs/b' ]"
    val=$(cat "$tmpdir/testrootfs/b")
    assertEquals 'package B content comes from a' "$val" "a"
    assertTrue 'package A installed' "[ -e '$tmpdir/testrootfs/a' ]"
}


testUnInstall() {
    [ "$LUET_BACKEND" == "img" ] && startSkipping
    luet uninstall -y --full --config $tmpdir/luet.yaml test/b
    installst=$?
    assertEquals 'uninstall test successfully' "$installst" "0"
    assertTrue 'package uninstalled' "[ ! -e '$tmpdir/testrootfs/b' ]"
    assertTrue 'package uninstalled' "[ ! -e '$tmpdir/testrootfs/a' ]"
}


testCleanup() {
    [ "$LUET_BACKEND" == "img" ] && startSkipping
    luet cleanup --config $tmpdir/luet.yaml
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    assertTrue 'package installed' "[ ! -e '$tmpdir/testrootfs/packages/b-test-1.0.package.tar.gz' ]"
}

# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2

