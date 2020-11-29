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
    luet build --tree "$ROOT_DIR/tests/fixtures/buildableseed" --destination $tmpdir/testbuild --compression gzip test/c #> /dev/null
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

testDatabase() {
    luet database create --config $tmpdir/luet.yaml $tmpdir/testbuild/c-test-1.0.metadata.yaml
    #luet install -y --config $tmpdir/luet.yaml test/c-1.0 > /dev/null
    createst=$?
    assertEquals 'created package successfully' "$createst" "0"
    assertTrue 'package not installed' "[ ! -e '$tmpdir/testrootfs/c' ]"

    installed=$(luet --config $tmpdir/luet.yaml search --installed .)
    searchst=$?
    assertEquals 'search exists successfully' "$searchst" "0"
    assertContains 'contains test/c-1.0' "$installed" 'test/c-1.0'
    touch $tmpdir/testrootfs/c
    
    luet database remove --config $tmpdir/luet.yaml test/c@1.0
    removetest=$?
    assertEquals 'package removed successfully' "$removetest" "0"
    assertTrue 'file not touched' "[ -e '$tmpdir/testrootfs/c' ]"

    luet database create --config $tmpdir/luet.yaml $tmpdir/testbuild/c-test-1.0.metadata.yaml
    #luet install -y --config $tmpdir/luet.yaml test/c-1.0 > /dev/null
    createst=$?
    assertEquals 'created package successfully' "$createst" "0"
    assertTrue 'file still present' "[ -e '$tmpdir/testrootfs/c' ]"
    
    luet uninstall -y --config $tmpdir/luet.yaml test/c
    installst=$?
    assertEquals 'uninstall test successfully' "$installst" "0"
    assertTrue 'package uninstalled' "[ ! -e '$tmpdir/testrootfs/c' ]"
}

# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2

