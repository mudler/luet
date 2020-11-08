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
    luet build --tree "$ROOT_DIR/tests/fixtures/upgrade_old_repo" --destination $tmpdir/testbuild --compression gzip --full --clean=true
    buildst=$?
    assertTrue 'create package B 1.0' "[ -e '$tmpdir/testbuild/b-test-1.0.package.tar.gz' ]"
    assertEquals 'builds successfully' "$buildst" "0"

    mkdir $tmpdir/testbuild_revision
    luet build --tree "$ROOT_DIR/tests/fixtures/upgrade_old_repo_revision" --destination $tmpdir/testbuild_revision --compression gzip --full --clean=true
    buildst=$?
    assertTrue 'create package B 1.0' "[ -e '$tmpdir/testbuild_revision/b-test-1.0.package.tar.gz' ]"
    assertEquals 'builds successfully' "$buildst" "0"
}

testRepo() {
    assertTrue 'no repository' "[ ! -e '$tmpdir/testbuild/repository.yaml' ]"
    luet create-repo --tree "$ROOT_DIR/tests/fixtures/upgrade_old_repo" \
    --output $tmpdir/testbuild \
    --packages $tmpdir/testbuild \
    --name "test" \
    --descr "Test Repo" \
    --urls $tmpdir/testrootfs \
    --type http

    createst=$?
    assertEquals 'create repo successfully' "$createst" "0"
    assertTrue 'create repository' "[ -e '$tmpdir/testbuild/repository.yaml' ]"

    assertTrue 'no repository' "[ ! -e '$tmpdir/testbuild_revision/repository.yaml' ]"
    luet create-repo --tree "$ROOT_DIR/tests/fixtures/upgrade_old_repo_revision" \
    --output $tmpdir/testbuild_revision \
    --packages $tmpdir/testbuild_revision \
    --name "test" \
    --descr "Test Repo" \
    --urls $tmpdir/testrootfs \
    --type http

    createst=$?
    assertEquals 'create repo successfully' "$createst" "0"
    assertTrue 'create repository' "[ -e '$tmpdir/testbuild_revision/repository.yaml' ]"
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

testUpgrade() {
    luet install --config $tmpdir/luet.yaml test/b-1.0
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    assertTrue 'package installed B' "[ -e '$tmpdir/testrootfs/test5' ]"

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
       - "$tmpdir/testbuild_revision"
EOF

    luet cleanup --config $tmpdir/luet.yaml
    luet config --config $tmpdir/luet.yaml
    res=$?
    assertEquals 'config test successfully' "$res" "0"

    luet upgrade --sync --config $tmpdir/luet.yaml
    installst=$?
    assertEquals 'upgrade test successfully' "$installst" "0"
    assertTrue 'package uninstalled B' "[ ! -e '$tmpdir/testrootfs/test5' ]"
    assertTrue 'package installed B' "[ -e '$tmpdir/testrootfs/newc' ]"

    content=$(luet upgrade --sync --config $tmpdir/luet.yaml)
    installst=$?
    assertNotContains 'didn not upgrade' "$content" "Uninstalling"
}


# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2

