#!/bin/bash

export LUET_NOLOCK=true

oneTimeSetUp() {
export tmpdir="$(mktemp -d)"
}

oneTimeTearDown() {
    rm -rf "$tmpdir"
}

testBuildA() {
    mkdir $tmpdir/testbuild1
    luet build --tree "$ROOT_DIR/tests/fixtures/virtuals"  --debug --compression "gzip" --destination $tmpdir/testbuild1 test/a
    buildst=$?
    assertEquals 'builds successfully' "$buildst" "0"
    assertTrue 'create package A 1.0' "[ -e '$tmpdir/testbuild1/a-test-1.0.package.tar.gz' ]"
    assertTrue 'create package A 1.0' "[ -e '$tmpdir/testbuild1/a-test-1.0.metadata.yaml' ]"
}

testBuildB() {
    mkdir $tmpdir/testbuild2
    luet build --tree "$ROOT_DIR/tests/fixtures/virtuals"  --debug --compression "gzip" --destination $tmpdir/testbuild2 test/b
    buildst=$?
    assertEquals 'builds successfully' "$buildst" "0"
    assertTrue 'create package A 1.0' "[ -e '$tmpdir/testbuild2/a-test-1.0.package.tar.gz' ]"
    assertTrue 'create package A 1.0' "[ -e '$tmpdir/testbuild2/a-test-1.0.metadata.yaml' ]"
    assertTrue 'create package B 1.0' "[ -e '$tmpdir/testbuild2/b-test-1.0.package.tar.gz' ]"
    assertTrue 'create package B 1.0' "[ -e '$tmpdir/testbuild2/b-test-1.0.metadata.yaml' ]"
}

testBuildC() {
    mkdir $tmpdir/testbuild3
    luet build --tree "$ROOT_DIR/tests/fixtures/virtuals"  --debug --destination $tmpdir/testbuild3 test/c
    buildst=$?
    assertEquals 'builds of C expected to fail' "$buildst" "1"
}

# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2

