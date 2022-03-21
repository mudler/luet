#!/bin/bash

export LUET_NOLOCK=true
oneTimeSetUp() {
export tmpdir="$(mktemp -d)"
}

oneTimeTearDown() {
    rm -rf "$tmpdir"
}

testBuild() {
    cat <<EOF > $tmpdir/default.yaml
bb: "ttt"
EOF
    mkdir $tmpdir/testbuild
    luet build --tree "$ROOT_DIR/tests/fixtures/build_values_nested" --values $tmpdir/default.yaml --destination $tmpdir/testbuild --compression gzip --all 
    buildst=$?
    assertEquals 'builds successfully' "$buildst" "0"
    assertTrue 'create package B' "[ -e '$tmpdir/testbuild/b-distro-0.3.package.tar.gz' ]"
    assertTrue 'create package A' "[ -e '$tmpdir/testbuild/a-distro-0.1.package.tar.gz' ]"
    assertTrue 'create package C' "[ -e '$tmpdir/testbuild/c-distro-0.3.package.tar.gz' ]"
    assertTrue 'create package foo' "[ -e '$tmpdir/testbuild/foo-test-1.1.package.tar.gz' ]"
}

testRepo() {
    assertTrue 'no repository' "[ ! -e '$tmpdir/testbuild/repository.yaml' ]"
    luet create-repo --tree "$ROOT_DIR/tests/fixtures/build_values_nested" \
    --output $tmpdir/testbuild \
    --packages $tmpdir/testbuild \
    --name "test" \
    --descr "Test Repo" \
    --urls $tmpdir/testrootfs \
    --type disk

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
    luet install -y --config $tmpdir/luet.yaml distro/a
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"

    assertTrue 'package installed A' "[ -e '$tmpdir/testrootfs/a' ]"
    # Build time can interpolate on fields which aren't package properties.
    assertTrue 'extra field on A' "[ -e '$tmpdir/testrootfs/build-extra-baz' ]"
    assertTrue 'package installed A interpolated with values' "[ -e '$tmpdir/testrootfs/a-ttt' ]"
    # Finalizers can interpolate only on package field. No extra fields are allowed at this time.
    assertTrue 'finalizer executed on A' "[ -e '$tmpdir/testrootfs/finalize-a' ]"

    installed=$(luet --config $tmpdir/luet.yaml search --installed .)
    searchst=$?
    assertEquals 'search exists successfully' "$searchst" "0"

    assertContains 'contains distro/a-0.1' "$installed" 'distro/a-0.1'

    luet uninstall -y --config $tmpdir/luet.yaml distro/a
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"

    # We do the same check for the others
    luet install -y --config $tmpdir/luet.yaml distro/b
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"

    assertTrue 'package installed B' "[ -e '$tmpdir/testrootfs/b' ]"
    assertTrue 'package installed B interpolated with values' "[ -e '$tmpdir/testrootfs/b-ttt' ]"
    assertTrue 'extra field on B' "[ -e '$tmpdir/testrootfs/build-extra-f' ]"
    assertTrue 'finalizer executed on B' "[ -e '$tmpdir/testrootfs/finalize-b' ]"

    installed=$(luet --config $tmpdir/luet.yaml search --installed .)
    searchst=$?
    assertEquals 'search exists successfully' "$searchst" "0"

    assertContains 'contains distro/b-0.3' "$installed" 'distro/b-0.3'

    luet uninstall -y --config $tmpdir/luet.yaml distro/b
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"

    luet install -y --config $tmpdir/luet.yaml distro/c
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"

    assertTrue 'package installed C' "[ -e '$tmpdir/testrootfs/c' ]"
    assertTrue 'extra field on C' "[ -e '$tmpdir/testrootfs/build-extra-bar' ]"
    assertTrue 'package installed C interpolated with values' "[ -e '$tmpdir/testrootfs/c-ttt' ]"
    assertTrue 'finalizer executed on C' "[ -e '$tmpdir/testrootfs/finalize-c' ]"

    installed=$(luet --config $tmpdir/luet.yaml search --installed .)
    searchst=$?
    assertEquals 'search exists successfully' "$searchst" "0"

    assertContains 'contains distro/c-0.3' "$installed" 'distro/c-0.3'

    luet uninstall -y --config $tmpdir/luet.yaml distro/c
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"

    luet install -y --config $tmpdir/luet.yaml test/foo
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"

    assertTrue 'package installed foo' "[ -e '$tmpdir/testrootfs/foo' ]"
    assertTrue 'package installed foo interpolated with values' "[ -e '$tmpdir/testrootfs/foo-ttt' ]"
}
# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2

