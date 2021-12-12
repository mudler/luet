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
    luet build --tree "$ROOT_DIR/tests/fixtures/upgrade_integration_oscheck" --destination $tmpdir/testbuild --compression gzip test/b@1.0
    buildst=$?
    assertTrue 'create package B 1.0' "[ -e '$tmpdir/testbuild/b-test-1.0.package.tar.gz' ]"
    assertEquals 'builds successfully' "$buildst" "0"

    luet build --tree "$ROOT_DIR/tests/fixtures/upgrade_integration_oscheck" --destination $tmpdir/testbuild --compression gzip test/b@1.1
    buildst=$?
    assertEquals 'builds successfully' "$buildst" "0"
    assertTrue 'create package B 1.1' "[ -e '$tmpdir/testbuild/b-test-1.1.package.tar.gz' ]"

    luet build --tree "$ROOT_DIR/tests/fixtures/upgrade_integration_oscheck" --destination $tmpdir/testbuild --compression gzip test/a@1.0 
    buildst=$?
    assertEquals 'builds successfully' "$buildst" "0"
    assertTrue 'create package A 1.0' "[ -e '$tmpdir/testbuild/a-test-1.0.package.tar.gz' ]"

    luet build --tree "$ROOT_DIR/tests/fixtures/upgrade_integration_oscheck" --destination $tmpdir/testbuild --compression gzip test/a@1.1
    buildst=$?
    assertEquals 'builds successfully' "$buildst" "0"

    assertTrue 'create package A 1.1' "[ -e '$tmpdir/testbuild/a-test-1.1.package.tar.gz' ]"

    luet build --tree "$ROOT_DIR/tests/fixtures/upgrade_integration_oscheck" --destination $tmpdir/testbuild --compression gzip test/a@1.2
    buildst=$?
    assertEquals 'builds successfully' "$buildst" "0"
    assertTrue 'create package A 1.2' "[ -e '$tmpdir/testbuild/a-test-1.2.package.tar.gz' ]"

    luet build --tree "$ROOT_DIR/tests/fixtures/upgrade_integration_oscheck" --destination $tmpdir/testbuild --compression gzip test/z@1.0
    buildst=$?
    assertEquals 'builds successfully' "$buildst" "0"
    assertTrue 'create package Z 1.0' "[ -e '$tmpdir/testbuild/z-test-1.0.package.tar.gz' ]"

    luet build --tree "$ROOT_DIR/tests/fixtures/upgrade_integration_oscheck" --destination $tmpdir/testbuild --compression gzip test/c@1.0
    buildst=$?
    assertEquals 'builds successfully' "$buildst" "0"
    assertTrue 'create package C 1.0' "[ -e '$tmpdir/testbuild/c-test-1.0.package.tar.gz' ]"

}

testRepo() {
    assertTrue 'no repository' "[ ! -e '$tmpdir/testbuild/repository.yaml' ]"
    luet create-repo --tree "$ROOT_DIR/tests/fixtures/upgrade_integration_oscheck" \
    --output $tmpdir/testbuild \
    --packages $tmpdir/testbuild \
    --name "test" \
    --descr "Test Repo" \
    --urls $tmpdir/testrootfs \
    --type http

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
    luet install -y --relax --config $tmpdir/luet.yaml test/b@1.0 
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    assertTrue 'package installed B' "[ -e '$tmpdir/testrootfs/test5' ]"

    luet install -y --relax --config $tmpdir/luet.yaml test/z@1.0
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    assertTrue 'package installed Z' "[ -e '$tmpdir/testrootfs/z' ]"

    luet install -y --relax --config $tmpdir/luet.yaml test/a@1.0
    assertTrue 'package installed A' "[ -e '$tmpdir/testrootfs/testaa' ]"
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"

    luet install -y --relax --config $tmpdir/luet.yaml test/c@1.0
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    assertTrue 'package installed C' "[ -e '$tmpdir/testrootfs/c' ]"
}

testUpgrade() {
    rm -rf $tmpdir/testrootfs/z
    assertTrue 'package Z corrupted' "[ ! -e '$tmpdir/testrootfs/z' ]"

    upgrade=$(luet --config $tmpdir/luet.yaml upgrade --oscheck -y)
    installst=$?
    echo "$upgrade"
    assertEquals 'install test successfully' "$installst" "0"
    assertTrue 'package uninstalled B' "[ ! -e '$tmpdir/testrootfs/test5' ]"
    assertTrue 'package installed B' "[ -e '$tmpdir/testrootfs/newc' ]"
    assertTrue 'package uninstalled A' "[ ! -e '$tmpdir/testrootfs/testaa' ]"
    assertTrue 'package Z restored' "[ -e '$tmpdir/testrootfs/z' ]"
    assertTrue 'package installed new A' "[ -e '$tmpdir/testrootfs/testlatest' ]"
    assertNotContains 'does not contain test/c@1.0' "$upgrade" 'test/c-1.0'
    assertNotContains 'does not attempt to download test/c@1.0' "$upgrade" 'test/c-1.0 downloaded'
}

# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2

