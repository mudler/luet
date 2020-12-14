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
    luet build --tree "$ROOT_DIR/tests/fixtures/symlinks" --destination $tmpdir/testbuild --compression gzip --full
    buildst=$?
    assertTrue 'create package pkgAsym 0.1' "[ -e '$tmpdir/testbuild/pkgAsym-test-0.1.package.tar.gz' ]"
    assertEquals 'builds successfully' "$buildst" "0"
}

testRepo() {
    assertTrue 'no repository' "[ ! -e '$tmpdir/testbuild/repository.yaml' ]"
    luet create-repo --tree "$ROOT_DIR/tests/fixtures/symlinks" \
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
    luet install -y --config $tmpdir/luet.yaml test/pkgAsym test/pkgBsym
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    ls -liah $tmpdir/testrootfs/
    assertTrue 'package did not install file2' "[ ! -e '$tmpdir/testrootfs/file2' ]"
    assertTrue 'package installed file3 as symlink' "[ -L '$tmpdir/testrootfs/file3' ]"
    assertTrue 'package installed file1 as symlink' "[ -L '$tmpdir/testrootfs/file1' ]"

}


# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2

