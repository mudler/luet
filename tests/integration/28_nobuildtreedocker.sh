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

testConfig() {
    [ -z "${TEST_DOCKER_IMAGE:-}" ] && startSkipping

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

testBuild() {
    [ -z "${TEST_DOCKER_IMAGE:-}" ] && startSkipping

    mkdir $tmpdir/testbuild
    mkdir $tmpdir/empty
    build_output=$(luet build --pull --tree "$tmpdir/empty" \
    --config $tmpdir/luet.yaml --concurrency 1 \
    --from-repositories --destination $tmpdir/testbuild --compression zstd test/c@1.0 test/z test/interpolated)
    buildst=$?
    echo "$build_output"
    assertEquals 'builds successfully' "$buildst" "0"
    assertTrue 'create package dep B' "[ -e '$tmpdir/testbuild/b-test-1.0.package.tar.zst' ]"
    assertTrue 'create package' "[ -e '$tmpdir/testbuild/c-test-1.0.package.tar.zst' ]"
    assertTrue 'create package Z' "[ -e '$tmpdir/testbuild/z-test-1.0+2.package.tar.zst' ]"
    assertTrue 'create package interpolated' "[ -e '$tmpdir/testbuild/interpolated-test-1.0+2.package.tar.zst' ]"
    assertContains 'Does use the upstream cache without specifying it test/c' "$build_output" "Images available remotely for test/c-1.0 generating artifact from remote images: quay.io/mocaccinoos/integration-test-cache:5387bd29accbd644df2b9d064c19451cd7a0ba57583a225af8ef76b79fb07511"
    assertContains 'Does use the upstream cache without specifying it test/z' "$build_output" "Images available remotely for test/z-1.0+2 generating artifact from remote images: quay.io/mocaccinoos/integration-test-cache:44aa6020c74536c8eb3bb501e0f69c68c63c071ebfb5da7c395655f78114ea83"
    assertContains 'Does use the upstream cache without specifying it test/interpolated' "$build_output" "Images available remotely for test/interpolated-1.0+2 generating artifact from remote images: quay.io/mocaccinoos/integration-test-cache:3229bfee7cb1774e92f9b669ecf6c97c58a70ecb941fa2b1d8a32198a75a76f0"
}

testRepo() {
    # Disable tests which require a DOCKER registry
    [ -z "${TEST_DOCKER_IMAGE:-}" ] && startSkipping

    luet create-repo \
    --output "${TEST_DOCKER_IMAGE}-2" \
    --packages $tmpdir/testbuild \
    --name "test" \
    --descr "Test Repo" \
    --urls $tmpdir/testrootfs \
    --tree-compression zstd \
    --tree-filename foo.tar \
    --tree "$tmpdir/empty" --config $tmpdir/luet.yaml --from-repositories \
    --meta-filename repository.meta.tar \
    --meta-compression zstd \
    --type docker --push-images --force-push --debug

    createst=$?
    assertEquals 'create repo successfully' "$createst" "0"
}

testConfigClient() {
    [ -z "${TEST_DOCKER_IMAGE:-}" ] && startSkipping

    cat <<EOF > $tmpdir/luet-client.yaml
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
       - "${TEST_DOCKER_IMAGE}-2"
EOF
    luet config --config $tmpdir/luet-client.yaml
    res=$?
    assertEquals 'config test successfully' "$res" "0"
}

testInstall() {
    # Disable tests which require a DOCKER registry
    [ -z "${TEST_DOCKER_IMAGE:-}" ] && startSkipping

    luet install -y --config $tmpdir/luet-client.yaml test/c@1.0 test/z test/interpolated
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    assertTrue 'package installed' "[ -e '$tmpdir/testrootfs/c' ]"
    assertTrue 'package Z installed' "[ -e '$tmpdir/testrootfs/z' ]"
    assertTrue 'package interpolated installed' "[ -e '$tmpdir/testrootfs/interpolated-baz-bar' ]"
}

testReInstall() {
    # Disable tests which require a DOCKER registry
    [ -z "${TEST_DOCKER_IMAGE:-}" ] && startSkipping

    output=$(luet install -y --config $tmpdir/luet-client.yaml  test/c@1.0)
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    assertContains 'contains warning' "$output" 'No packages to install'
}

testUnInstall() {
    # Disable tests which require a DOCKER registry
    [ -z "${TEST_DOCKER_IMAGE:-}" ] && startSkipping

    luet uninstall -y --config $tmpdir/luet-client.yaml test/c@1.0
    installst=$?
    assertEquals 'uninstall test successfully' "$installst" "0"
    assertTrue 'package uninstalled' "[ ! -e '$tmpdir/testrootfs/c' ]"
}

testInstallAgain() {
    # Disable tests which require a DOCKER registry
    [ -z "${TEST_DOCKER_IMAGE:-}" ] && startSkipping

    assertTrue 'package uninstalled' "[ ! -e '$tmpdir/testrootfs/c' ]"
    output=$(luet install -y --config $tmpdir/luet-client.yaml test/c@1.0)
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
    assertNotContains 'contains warning' "$output" 'No packages to install'
    assertTrue 'package installed' "[ -e '$tmpdir/testrootfs/c' ]"
    assertTrue 'cache initialized' "[ -e '$tmpdir/testrootfs/packages/README' ]"
}

testCleanup() {
    [ -z "${TEST_DOCKER_IMAGE:-}" ] && startSkipping

    luet cleanup --config $tmpdir/luet-client.yaml
    installst=$?
    assertEquals 'cleanup test successfully' "$installst" "0"
    assertTrue 'cache removed' "[ ! -e '$tmpdir/testrootfs/packages/README' ]"
}

# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2

