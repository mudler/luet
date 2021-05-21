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
    [ "$LUET_BACKEND" == "img" ] && startSkipping
    mkdir $tmpdir/testbuild
    luet build --tree "$ROOT_DIR/tests/fixtures/join_complex" \
               --destination $tmpdir/testbuild --concurrency 1 \
               --compression gzip \
               test/z test/x
    buildst=$?
    assertEquals 'builds successfully' "$buildst" "0"
    assertTrue 'create package z' "[ -e '$tmpdir/testbuild/z-test-0.1.package.tar.gz' ]"
    assertTrue 'create package z' "[ -e '$tmpdir/testbuild/x-test-0.1.package.tar.gz' ]"

    mkdir $tmpdir/extract
    tar -xvf $tmpdir/testbuild/x-test-0.1.package.tar.gz -C $tmpdir/extract
    tar -xvf $tmpdir/testbuild/z-test-0.1.package.tar.gz -C $tmpdir/extract
    assertTrue 'create result from a package that requires a join' "[ -e '$tmpdir/extract/z' ]"
    assertTrue 'create result from join of a join' "[ -e '$tmpdir/extract/x' ]"
}


# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2

