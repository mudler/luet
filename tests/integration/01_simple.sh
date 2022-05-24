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
    luet build --tree "$ROOT_DIR/tests/fixtures/buildableseed" --destination $tmpdir/testbuild --compression gzip test/c > /dev/null
    buildst=$?
    assertEquals 'builds successfully' "$buildst" "0"
    assertTrue 'create package dep B' "[ -e '$tmpdir/testbuild/b-test-1.0.package.tar.gz' ]"
    assertTrue 'create package' "[ -e '$tmpdir/testbuild/c-test-1.0.package.tar.gz' ]"
}

testRepo() {
    assertTrue 'no repository' "[ ! -e '$tmpdir/testbuild/repository.yaml' ]"
    luet create-repo --tree "$ROOT_DIR/tests/fixtures/buildableseed" \
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
    luet install -y --config $tmpdir/luet.yaml test/foobar
    installst=$?
    assertEquals 'install test fails' "$installst" "2"

    luet install -y --config $tmpdir/luet.yaml test/foobar test/c
    installst=$?
    assertEquals 'install test fails' "$installst" "2"

    luet install -y --config $tmpdir/luet.yaml test/foobar@1.0
    installst=$?
    assertEquals 'install test fails' "$installst" "2"

    luet install -y --config $tmpdir/luet.yaml test/foobar@1.0 test/c@1.0
    installst=$?
    assertEquals 'install test fails' "$installst" "2"

    luet install -y --config $tmpdir/luet.yaml test/foobar@1.0 test/c
    installst=$?
    assertEquals 'install test fails' "$installst" "2"

    luet install -y --config $tmpdir/luet.yaml test/c
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    assertTrue 'package installed' "[ -e '$tmpdir/testrootfs/c' ]"

    luet install -y --config $tmpdir/luet.yaml test/foobar test/c
    installst=$?
    assertEquals 'install test fails' "$installst" "2"

    # Already installed
    luet install -y --config $tmpdir/luet.yaml test/c@1.0
    installst=$?
    assertEquals 'install test fails' "$installst" "0"
}

testReInstall() {
    output=$(luet install -y --config $tmpdir/luet.yaml  test/c@1.0)
    echo "$output"
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    assertContains 'contains warning' "$output" 'No packages to install'
}

testUnInstall() {
    luet uninstall -y --config $tmpdir/luet.yaml test/c
    installst=$?
    assertEquals 'uninstall test successfully' "$installst" "0"
    assertTrue 'package uninstalled' "[ ! -e '$tmpdir/testrootfs/c' ]"
}

testInstallAgain() {
    assertTrue 'package uninstalled' "[ ! -e '$tmpdir/testrootfs/c' ]"
    output=$(luet install -y --config $tmpdir/luet.yaml test/c@1.0)
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    assertNotContains 'contains warning' "$output" 'No packages to install'
    assertTrue 'package installed' "[ -e '$tmpdir/testrootfs/c' ]"
}

testCleanup() {
    luet cleanup --config $tmpdir/luet.yaml
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    assertTrue 'package installed' "[ ! -e '$tmpdir/testrootfs/packages/c-test-1.0.package.tar.gz' ]"
}

# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2

