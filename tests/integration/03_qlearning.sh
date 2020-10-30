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
    luet build --all --concurrency 1 --tree "$ROOT_DIR/tests/fixtures/qlearning" --destination $tmpdir/testbuild --compression gzip
    buildst=$?
    assertEquals 'builds successfully' "$buildst" "0"
    assertTrue 'create package dep B' "[ -e '$tmpdir/testbuild/b-test-1.0.package.tar.gz' ]"
    assertTrue 'create package' "[ -e '$tmpdir/testbuild/c-test-1.0.package.tar.gz' ]"
}

testRepo() {
    assertTrue 'no repository' "[ ! -e '$tmpdir/testbuild/repository.yaml' ]"
    luet create-repo --tree "$ROOT_DIR/tests/fixtures/qlearning" \
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
    luet install --config $tmpdir/luet.yaml test/c
    #luet install --config $tmpdir/luet.yaml test/c-1.0 > /dev/null
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    assertTrue 'package C installed' "[ -e '$tmpdir/testrootfs/c' ]"
}

testFullInstall() {
    output=$(luet install --config $tmpdir/luet.yaml test/d test/f test/e test/a)
    installst=$?
    assertEquals 'cannot install' "$installst" "1"
    assertTrue 'package D installed' "[ ! -e '$tmpdir/testrootfs/d' ]"
    assertTrue 'package F installed' "[ ! -e '$tmpdir/testrootfs/f' ]"
}

testInstallAgain() {
    output=$(luet install --solver-type qlearning --config $tmpdir/luet.yaml test/d test/f test/e test/a)
    installst=$?
    echo "$output"
    assertEquals 'install test successfully' "0" "$installst"
    assertNotContains 'contains warning' "$output" 'Filtering out'
    assertTrue 'package D installed' "[ -e '$tmpdir/testrootfs/d' ]"
    assertTrue 'package F installed' "[ -e '$tmpdir/testrootfs/f' ]"
    assertTrue 'package E not installed' "[ ! -e '$tmpdir/testrootfs/e' ]"
    assertTrue 'package A not installed' "[ ! -e '$tmpdir/testrootfs/a' ]"
}

testCleanup() {
    luet cleanup --config $tmpdir/luet.yaml
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    assertTrue 'package installed' "[ ! -e '$tmpdir/testrootfs/packages/c-test-1.0.package.tar.gz' ]"
}

# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2

