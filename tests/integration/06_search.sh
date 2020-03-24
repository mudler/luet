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
    luet build --tree "$ROOT_DIR/tests/fixtures/upgrade_integration" --destination $tmpdir/testbuild --compression gzip test/b-1.0 
    buildst=$?
    assertTrue 'create package B 1.0' "[ -e '$tmpdir/testbuild/b-test-1.0.package.tar.gz' ]"
    assertEquals 'builds successfully' "$buildst" "0"

    luet build --tree "$ROOT_DIR/tests/fixtures/upgrade_integration" --destination $tmpdir/testbuild --compression gzip test/b-1.1
    buildst=$?
    assertEquals 'builds successfully' "$buildst" "0"
    assertTrue 'create package B 1.1' "[ -e '$tmpdir/testbuild/b-test-1.1.package.tar.gz' ]"

    luet build --tree "$ROOT_DIR/tests/fixtures/upgrade_integration" --destination $tmpdir/testbuild --compression gzip test/a-1.0 
    buildst=$?
    assertEquals 'builds successfully' "$buildst" "0"
    assertTrue 'create package A 1.0' "[ -e '$tmpdir/testbuild/a-test-1.0.package.tar.gz' ]"

    luet build --tree "$ROOT_DIR/tests/fixtures/upgrade_integration" --destination $tmpdir/testbuild --compression gzip test/a-1.1
    buildst=$?
    assertEquals 'builds successfully' "$buildst" "0"

    assertTrue 'create package A 1.1' "[ -e '$tmpdir/testbuild/a-test-1.1.package.tar.gz' ]"

    luet build --tree "$ROOT_DIR/tests/fixtures/upgrade_integration" --destination $tmpdir/testbuild --compression gzip test/a-1.2
    buildst=$?
    assertEquals 'builds successfully' "$buildst" "0"

    assertTrue 'create package A 1.2' "[ -e '$tmpdir/testbuild/a-test-1.2.package.tar.gz' ]"


    luet build --tree "$ROOT_DIR/tests/fixtures/upgrade_integration" --destination $tmpdir/testbuild --compression gzip test/c-1.0
    buildst=$?
    assertEquals 'builds successfully' "$buildst" "0"
    assertTrue 'create package C 1.0' "[ -e '$tmpdir/testbuild/c-test-1.0.package.tar.gz' ]"

}

testRepo() {
    assertTrue 'no repository' "[ ! -e '$tmpdir/testbuild/repository.yaml' ]"
    luet create-repo --tree "$ROOT_DIR/tests/fixtures/upgrade_integration" \
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
    luet install --config $tmpdir/luet.yaml test/b-1.0
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    assertTrue 'package installed B' "[ -e '$tmpdir/testrootfs/test5' ]"

    luet install --config $tmpdir/luet.yaml test/a-1.0
    assertTrue 'package installed A' "[ -e '$tmpdir/testrootfs/testaa' ]"
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"

    luet install --config $tmpdir/luet.yaml test/a-1.1
    assertTrue 'package installed A' "[ -e '$tmpdir/testrootfs/testaa' ]"
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    assertTrue 'package keeps old A' "[ -e '$tmpdir/testrootfs/testaa' ]"
    assertTrue 'package new A was not installed' "[ ! -e '$tmpdir/testrootfs/testlatest' ]"

    luet install --config $tmpdir/luet.yaml test/c-1.0
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    assertTrue 'package installed C' "[ -e '$tmpdir/testrootfs/c' ]"
}

testSearch() {
    installed=$(luet --config $tmpdir/luet.yaml search --installed .)
    searchst=$?
    assertEquals 'search exists successfully' "$searchst" "0"

    assertContains 'contains test/b-1.0' "$installed" 'test b 1.0'
    assertContains 'contains test/b-1.0' "$installed" 'test a 1.0'
    assertContains 'contains test/b-1.0' "$installed" 'test c 1.0'
}

# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2

