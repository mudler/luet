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
    cat <<EOF > $tmpdir/default.yaml
extra: "bar"
foo: "baz"
EOF
    mkdir $tmpdir/testbuild
    luet build --tree "$ROOT_DIR/tests/fixtures/copy" \
               --destination $tmpdir/testbuild --concurrency 1 \
               --compression gzip --values $tmpdir/default.yaml \
               test/c
    buildst=$?
    assertEquals 'builds successfully' "$buildst" "0"
    assertTrue 'create package c' "[ -e '$tmpdir/testbuild/c-test-1.2.package.tar.gz' ]"
    mkdir $tmpdir/extract
    tar -xvf $tmpdir/testbuild/c-test-1.2.package.tar.gz -C $tmpdir/extract
    assertTrue 'create result in package c' "[ -e '$tmpdir/extract/result' ]"
    assertTrue 'create busybox in package c' "[ -f '$tmpdir/extract/bina/busybox' ]"
}


# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2

