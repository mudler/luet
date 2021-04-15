#!/bin/bash

export LUET_NOLOCK=true

oneTimeSetUp() {
export tmpdir="$(mktemp -d)"
}

oneTimeTearDown() {
    echo "keeping $tmpdir"
    rm -rf "$tmpdir"
}

testBuild() {
    mkdir $tmpdir/testbuild
    luet build --tree "$ROOT_DIR/tests/fixtures/buildableseed" --destination $tmpdir/testbuild --compression gzip test/c
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
  database_engine: "memory"
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

testBuildWithNoTree() {
    mkdir $tmpdir/testbuild2
    mkdir $tmpdir/emptytree
    luet build --from-repositories --tree $tmpdir/emptytree --config $tmpdir/luet.yaml test/c --destination $tmpdir/testbuild2
    buildst=$?
    assertEquals 'build test successfully' "$buildst" "0"
    assertTrue 'create package' "[ -e '$tmpdir/testbuild2/c-test-1.0.package.tar' ]"
}

testRepo2() {
    assertTrue 'no repository' "[ ! -e '$tmpdir/testbuild2/repository.yaml' ]"
    luet create-repo --config $tmpdir/luet.yaml --from-repositories --tree $tmpdir/emptytree \
    --output $tmpdir/testbuild2 \
    --packages $tmpdir/testbuild2 \
    --name "test" \
    --descr "Test Repo" \
    --urls $tmpdir/testrootfs \
    --type disk

    createst=$?
    assertEquals 'create repo successfully' "$createst" "0"
    assertTrue 'create repository' "[ -e '$tmpdir/testbuild2/repository.yaml' ]"
}

testCleanup() {
    luet cleanup --config $tmpdir/luet.yaml
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    assertTrue 'package cleaned' "[ ! -e '$tmpdir/testrootfs/packages/c-test-1.0.package.tar.gz' ]"
}

testInstall2() {

    cat <<EOF > $tmpdir/luet2.yaml
general:
  debug: true
system:
  rootfs: $tmpdir/testrootfs
  database_engine: "memory"
config_from_host: true
repositories:
   - name: "main"
     type: "disk"
     enable: true
     urls:
       - "$tmpdir/testbuild2"
EOF
    luet install -y --config $tmpdir/luet2.yaml --system-target $tmpdir/foo test/c
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    assertTrue 'db not created' "[ ! -e '$tmpdir/foo/var/cache/luet/luet.db' ]"
    assertTrue 'package installed' "[ -e '$tmpdir/foo/c' ]"
}

# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2

