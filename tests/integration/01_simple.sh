#!/bin/bash
set -e
export LUET_NOLOCK=true

oneTimeSetUp() {
export tmpdir="$(mktemp -d)"
}

oneTimeTearDown() {
    rm -rf "$tmpdir"
}

testBuild() {
    mkdir $tmpdir/testbuild
    luet build --tree "$ROOT_DIR/tests/fixtures/buildableseed" --destination $tmpdir/testbuild --compression gzip test/c-1.0
    buildst=$?
    assertEquals 'builds successfully' "$buildst" "0"
}

testRepo() {
    luet create-repo --tree "$ROOT_DIR/tests/fixtures/buildableseed" \
    --output $tmpdir/testbuild \
    --packages $tmpdir/testbuild \
    --name "test" \
    --uri $tmpdir/testrootfs \
    --type local

    createst=$?
    assertEquals 'create repo successfully' "$createst" "0"
}

testInstall() {
    mkdir $tmpdir/testrootfs
    cat <<EOF > $tmpdir/luet.yaml  
system-repositories: 
                   - name: "main"
                     type: "local"
                     uri: "$tmpdir/testbuild"
EOF
    luet install --config $tmpdir/luet.yaml --system-dbpath $tmpdir/testrootfs --system-target $tmpdir/testrootfs test/c-1.0
    installst=$?
    assertEquals 'install test successfully' "$installst" "0"
}


# Load shUnit2.
. "$ROOT_DIR/tests/integration/shunit2"/shunit2

