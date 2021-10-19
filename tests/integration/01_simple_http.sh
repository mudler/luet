#!/bin/bash

export LUET_NOLOCK=true
TEST_PORT="${TEST_PORT:-9090}"

oneTimeSetUp() {
export tmpdir="$(mktemp -d)"
}

oneTimeTearDown() {
    rm -rf "$tmpdir"
    kill '%1' || true
}

testBuild() {
    mkdir $tmpdir/testbuild
    luet build --tree "$ROOT_DIR/tests/fixtures/buildableseed" --destination $tmpdir/testbuild --compression zstd test/c@1.0 > /dev/null
    buildst=$?
    assertEquals 'builds successfully' "$buildst" "0"
    assertTrue 'create package dep B' "[ -e '$tmpdir/testbuild/b-test-1.0.package.tar.zst' ]"
    assertTrue 'create package' "[ -e '$tmpdir/testbuild/c-test-1.0.package.tar.zst' ]"
}

testRepo() {
    luet create-repo --tree "$ROOT_DIR/tests/fixtures/buildableseed" \
    --output $tmpdir/testbuild \
    --packages $tmpdir/testbuild \
    --name "test" \
    --descr "Test Repo" \
    --urls $tmpdir/testrootfs \
    --tree-compression zstd \
    --tree-filename foo.tar \
    --meta-filename repository.meta.tar \
    --meta-compression zstd \
    --type disk

    createst=$?
    assertEquals 'create repo successfully' "$createst" "0"

    luet serve-repo --dir $tmpdir/testbuild --port $TEST_PORT &
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
     type: "http"
     enable: true
     urls:
       - "http://127.0.0.1:$TEST_PORT"
EOF
    luet config --config $tmpdir/luet.yaml
    res=$?
    assertEquals 'config test successfully' "$res" "0"
}

testInstall() {
    luet install -y --config $tmpdir/luet.yaml test/c@1.0
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    assertTrue 'package installed' "[ -e '$tmpdir/testrootfs/c' ]"
}

testReInstall() {
    output=$(luet install -y --config $tmpdir/luet.yaml  test/c@1.0)
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    assertContains 'contains warning' "$output" 'No packages to install'
}

testUnInstall() {
    luet uninstall -y --config $tmpdir/luet.yaml test/c@1.0
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
    assertEquals 'cleanup test successfully' "$installst" "0"
}

# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2

