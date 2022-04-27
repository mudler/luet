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
    luet build --tree "$ROOT_DIR/tests/fixtures/subpackage" --destination $tmpdir/testbuild --compression gzip test/alpine
    buildst=$?
    assertEquals 'builds successfully' "$buildst" "0"
    assertTrue 'create package baz' "[ -e '$tmpdir/testbuild/baz-test-1.1.package.tar.gz' ]"
    assertTrue 'create package bar' "[ -e '$tmpdir/testbuild/bar-test-1.1.package.tar.gz' ]"
    assertTrue 'create package foo' "[ -e '$tmpdir/testbuild/foo-test-1.1.package.tar.gz' ]"
    assertTrue 'create package alpine' "[ -e '$tmpdir/testbuild/alpine-test-1.0.package.tar.gz' ]"
}

testRepo() {
    assertTrue 'no repository' "[ ! -e '$tmpdir/testbuild/repository.yaml' ]"
    luet create-repo --tree "$ROOT_DIR/tests/fixtures/subpackage" \
    --output $tmpdir/testbuild \
    --packages $tmpdir/testbuild \
    --name "test" \
    --descr "Test Repo" \
    --urls $tmpdir/testrootfs \
    --type disk

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
    luet install -y --config $tmpdir/luet.yaml test/foo
    #luet install -y --config $tmpdir/luet.yaml test/c@1.0 > /dev/null
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    assertTrue 'package not installed' "[ ! -d '$tmpdir/testrootfs/var' ]"
    assertTrue 'package installed' "[ -e '$tmpdir/testrootfs/bin/busybox' ]"
}

# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2

