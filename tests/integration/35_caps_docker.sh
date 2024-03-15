#!/bin/bash

export LUET_NOLOCK=true

oneTimeSetUp() {
export tmpdir="$(mktemp -d)"
}

oneTimeTearDown() {
    rm -rf "$tmpdir"
}

testBuild() {
    [ -z "${TEST_DOCKER_IMAGE:-}" ] && startSkipping
    [ "$LUET_BACKEND" == "img" ] && startSkipping

    mkdir $tmpdir/testbuild
    luet build -d --tree "$ROOT_DIR/tests/fixtures/caps" --same-owner=true --destination $tmpdir/testbuild --compression gzip --full
    buildst=$?
    assertTrue 'create package caps 0.1' "[ -e '$tmpdir/testbuild/caps-test-0.1.package.tar.gz' ]"
    assertEquals 'builds successfully' "$buildst" "0"
}

testRepo() {
    [ -z "${TEST_DOCKER_IMAGE:-}" ] && startSkipping
    [ "$LUET_BACKEND" == "img" ] && startSkipping

    assertTrue 'no repository' "[ ! -e '$tmpdir/testbuild/repository.yaml' ]"
    luet create-repo --tree "$ROOT_DIR/tests/fixtures/caps" \
    --output $TEST_DOCKER_IMAGE \
    --packages $tmpdir/testbuild \
    --name "test" \
    --descr "Test Repo" \
    --push-images --force-push \
    --type docker

    createst=$?
    assertEquals 'create repo successfully' "$createst" "0"
}

testConfig() {
    [ -z "${TEST_DOCKER_IMAGE:-}" ] && startSkipping
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
     type: "docker"
     enable: true
     urls:
       - "$TEST_DOCKER_IMAGE"
EOF
    luet config --config $tmpdir/luet.yaml
    res=$?
    assertEquals 'config test successfully' "$res" "0"
}

testInstall() {
    [ -z "${TEST_DOCKER_IMAGE:-}" ] && startSkipping
    [ "$LUET_BACKEND" == "img" ] && startSkipping

    $ROOT_DIR/tests/integration/bin/luet install -y --config $tmpdir/luet.yaml test/caps@0.1 test/caps2@0.1 test/empty
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
   
    assertTrue 'package installed file1' "[ -e '$tmpdir/testrootfs/file1' ]"
    assertTrue 'package installed file2' "[ -e '$tmpdir/testrootfs/file2' ]"

    getcap $tmpdir/testrootfs/file1
    getcap $tmpdir/testrootfs/file2
    #assertContains 'caps' "$(getcap $tmpdir/testrootfs/file1)" "cap_net_raw+ep"
    #assertContains 'caps' "$(getcap $tmpdir/testrootfs/file2)" "cap_net_raw+ep"
}


# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2

