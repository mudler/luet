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
    luet build --tree "$ROOT_DIR/tests/fixtures/shared_templates" --destination $tmpdir/testbuild --compression gzip --all
    buildst=$?
    assertEquals 'builds successfully' "$buildst" "0"
    assertTrue 'create packages' "[ -e '$tmpdir/testbuild/foo-test-1.0.package.tar.gz' ]"
    assertTrue 'create packages' "[ -e '$tmpdir/testbuild/bar-test-1.0.package.tar.gz' ]"
}

testRepo() {
    assertTrue 'no repository' "[ ! -e '$tmpdir/testbuild/repository.yaml' ]"
    luet create-repo --tree "$ROOT_DIR/tests/fixtures/shared_templates" \
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
    luet install -y --config $tmpdir/luet.yaml test/foo test/bar
    installst=$?
    assertEquals 'install test failed' "$installst" "0"
    assertTrue 'package bar installed' "[ -e '$tmpdir/testrootfs/bar' ]"
    assertTrue 'package foo installed' "[ -e '$tmpdir/testrootfs/foo' ]"
}

# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2

