#!/bin/bash

export LUET_NOLOCK=true

oneTimeSetUp() {
export tmpdir="$(mktemp -d)"
}

oneTimeTearDown() {
    rm -rf "$tmpdir"
}

testBuild() {
    [ "$LUET_BACKEND" == "img" ] && startSkipping
    mkdir $tmpdir/testbuild
    luet build -d --tree "$ROOT_DIR/tests/fixtures/extra_perms" --same-owner=true --destination $tmpdir/testbuild --compression gzip --full
    buildst=$?
    assertTrue 'create package perms 0.1' "[ -e '$tmpdir/testbuild/extra-perms-test-0.1.package.tar.gz' ]"
    assertEquals 'builds successfully' "$buildst" "0"
}

testRepo() {
    [ "$LUET_BACKEND" == "img" ] && startSkipping
    assertTrue 'no repository' "[ ! -e '$tmpdir/testbuild/repository.yaml' ]"
    luet create-repo --tree "$ROOT_DIR/tests/fixtures/extra_perms" \
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
    [ "$LUET_BACKEND" == "img" ] && startSkipping
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
    [ "$LUET_BACKEND" == "img" ] && startSkipping
    $ROOT_DIR/tests/integration/bin/luet install -y --config $tmpdir/luet.yaml test/extra-perms
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
   
    tree $tmpdir/testrootfs/foo/bar
    assertTrue 'package installed bar' "[ -d '$tmpdir/testrootfs/foo/bar' ]"

    assertContains 'perms2' "$(stat -c %u:%g $tmpdir/testrootfs/foo/bar)" "100:100"
    assertContains 'suid' "$(stat -c %a $tmpdir/testrootfs/foo/bar/suid)" "4644"
    assertContains 'sgid' "$(stat -c %a $tmpdir/testrootfs/foo/bar/sgid)" "2644"
    assertContains 'sticky' "$(stat -c %a $tmpdir/testrootfs/foo/bar/sticky)" "1644"
}


# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2

