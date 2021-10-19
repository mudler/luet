#!/bin/bash

export LUET_NOLOCK=true

oneTimeSetUp() {
    export tmpdir="$(mktemp -d)"
    docker images --filter='reference=luet/cache' --format='{{.Repository}}:{{.Tag}}' | xargs -r docker rmi
}

oneTimeTearDown() {
    rm -rf "$tmpdir"
    docker images --filter='reference=luet/cache' --format='{{.Repository}}:{{.Tag}}' | xargs -r docker rmi
}

testBuild() {
    # Disable tests which require a DOCKER registry
    [ -z "${TEST_DOCKER_IMAGE:-}" ] && startSkipping
    cat <<EOF > $tmpdir/default.yaml
extra: "bar"
foo: "baz"
EOF
    mkdir $tmpdir/testbuild
    luet build --tree "$ROOT_DIR/tests/fixtures/docker_repo" \
               --destination $tmpdir/testbuild --concurrency 1 \
               --image-repository "${TEST_DOCKER_IMAGE}-cache" --push \
               --compression zstd --values $tmpdir/default.yaml \
               test/c@1.0 test/z test/interpolated #> /dev/null
    buildst=$?
    assertEquals 'builds successfully' "$buildst" "0"
    assertTrue 'create package dep B' "[ -e '$tmpdir/testbuild/b-test-1.0.package.tar.zst' ]"
    assertTrue 'create package' "[ -e '$tmpdir/testbuild/c-test-1.0.package.tar.zst' ]"
    assertTrue 'create package z' "[ -e '$tmpdir/testbuild/z-test-1.0+2.package.tar.zst' ]"
    assertTrue 'create package interpolated' "[ -e '$tmpdir/testbuild/interpolated-test-1.0+2.package.tar.zst' ]"
}

testRepo() {
    # Disable tests which require a DOCKER registry
    [ -z "${TEST_DOCKER_IMAGE:-}" ] && startSkipping

    createres=$(luet create-repo --tree "$ROOT_DIR/tests/fixtures/docker_repo" \
    --output "${TEST_DOCKER_IMAGE}" \
    --packages $tmpdir/testbuild \
    --name "test" \
    --descr "Test Repo" \
    --urls $tmpdir/testrootfs \
    --tree-compression zstd \
    --tree-filename foo.tar \
    --meta-filename repository.meta.tar \
    --meta-compression zstd \
    --type docker --push-images --force-push)

    createst=$?
    assertEquals 'create repo successfully' "$createst" "0"
    assertContains 'contains image push' "$createres" 'Pushed image: quay.io/mocaccinoos/integration-test:z-test-1.0-2'
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
     type: "docker"
     enable: true
     urls:
       - "${TEST_DOCKER_IMAGE}"
EOF
    luet config --config $tmpdir/luet.yaml
    res=$?
    assertEquals 'config test successfully' "$res" "0"
}

testInstall() {
    # Disable tests which require a DOCKER registry
    [ -z "${TEST_DOCKER_IMAGE:-}" ] && startSkipping

    luet install -y --config $tmpdir/luet.yaml test/c@1.0 test/z test/interpolated
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    assertTrue 'package installed' "[ -e '$tmpdir/testrootfs/c' ]"
    assertTrue 'package Z installed' "[ -e '$tmpdir/testrootfs/z' ]"
    assertTrue 'package interpolated installed' "[ -e '$tmpdir/testrootfs/interpolated-baz-bar' ]"
}

testReInstall() {
    # Disable tests which require a DOCKER registry
    [ -z "${TEST_DOCKER_IMAGE:-}" ] && startSkipping

    output=$(luet install -y --config $tmpdir/luet.yaml  test/c@1.0)
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    assertContains 'contains warning' "$output" 'No packages to install'
}

testUnInstall() {
    # Disable tests which require a DOCKER registry
    [ -z "${TEST_DOCKER_IMAGE:-}" ] && startSkipping

    luet uninstall -y --config $tmpdir/luet.yaml test/c@1.0 test/z
    installst=$?
    assertEquals 'uninstall test successfully' "$installst" "0"
    assertTrue 'package uninstalled' "[ ! -e '$tmpdir/testrootfs/c' ]"
    assertTrue 'package Z uninstalled' "[ ! -e '$tmpdir/testrootfs/z' ]"
}

testInstallAgain() {
    # Disable tests which require a DOCKER registry
    [ -z "${TEST_DOCKER_IMAGE:-}" ] && startSkipping

    assertTrue 'package uninstalled' "[ ! -e '$tmpdir/testrootfs/c' ]"
    output=$(luet install -y --config $tmpdir/luet.yaml test/c@1.0 test/z)
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    assertNotContains 'contains warning' "$output" 'No packages to install'
    assertTrue 'package installed' "[ -e '$tmpdir/testrootfs/c' ]"
    assertTrue 'package Z installed' "[ -e '$tmpdir/testrootfs/z' ]"
}

testCleanup() {
    luet cleanup --config $tmpdir/luet.yaml
    installst=$?
    assertEquals 'cleanup test successfully' "$installst" "0"
}

# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2

