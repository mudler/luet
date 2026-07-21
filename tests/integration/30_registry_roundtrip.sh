#!/bin/bash

export LUET_NOLOCK=true

# Exercises a full build -> create-repo -> push -> pull -> install cycle
# against a local registry. Docker 29 pushes single-platform images as OCI
# indexes rather than plain manifests (moby/moby#51532); this test is what
# catches that, since the quay-backed docker tests skip without credentials.

REGISTRY_PORT=5000
REGISTRY_NAME=luet-test-registry
LOCAL_IMAGE="localhost:${REGISTRY_PORT}/luet-roundtrip"

oneTimeSetUp() {
    export tmpdir="$(mktemp -d)"
    docker rm -f "$REGISTRY_NAME" >/dev/null 2>&1
    docker run -d --name "$REGISTRY_NAME" \
        -p "${REGISTRY_PORT}:5000" registry:2 >/dev/null

    # Wait for the registry to accept connections.
    for _ in $(seq 1 30); do
        if curl -sf "http://localhost:${REGISTRY_PORT}/v2/" >/dev/null; then
            break
        fi
        sleep 1
    done
}

oneTimeTearDown() {
    docker rm -f "$REGISTRY_NAME" >/dev/null 2>&1
    rm -rf "$tmpdir"
}

testRegistryUp() {
    curl -sf "http://localhost:${REGISTRY_PORT}/v2/" >/dev/null
    assertEquals 'local registry is reachable' "0" "$?"
}

testBuild() {
    mkdir -p "$tmpdir/testbuild"
    cat <<EOF > "$tmpdir/default.yaml"
extra: "bar"
foo: "baz"
EOF
    luet build --tree "$ROOT_DIR/tests/fixtures/docker_repo" \
               --destination "$tmpdir/testbuild" --concurrency 1 \
               --image-repository "${LOCAL_IMAGE}-cache" --push \
               --compression zstd --values "$tmpdir/default.yaml" \
               test/c@1.0 test/z test/interpolated
    assertEquals 'builds and pushes cache images successfully' "0" "$?"
    assertTrue 'created package c' "[ -e '$tmpdir/testbuild/c-test-1.0.package.tar.zst' ]"
}

testCreateRepoAndPush() {
    luet create-repo --tree "$ROOT_DIR/tests/fixtures/docker_repo" \
        --output "${LOCAL_IMAGE}" \
        --packages "$tmpdir/testbuild" \
        --name "test" \
        --descr "Test Repo" \
        --urls "$tmpdir/testrootfs" \
        --tree-compression zstd \
        --tree-filename foo.tar \
        --meta-filename repository.meta.tar \
        --meta-compression zstd \
        --type docker --push-images --force-push
    assertEquals 'pushes repository to local registry' "0" "$?"
}

# The pull side is daemonless (go-containerregistry remote), so this is
# where an unexpected OCI index shape surfaces.
testInstallFromRegistry() {
    mkdir -p "$tmpdir/testrootfs"
    cat <<EOF > "$tmpdir/luet.yaml"
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
       - "${LOCAL_IMAGE}"
EOF
    luet install -y --config "$tmpdir/luet.yaml" test/c@1.0 test/z
    assertEquals 'installs from the local registry' "0" "$?"
    assertTrue 'package c installed' "[ -e '$tmpdir/testrootfs/c' ]"
    assertTrue 'package z installed' "[ -e '$tmpdir/testrootfs/z' ]"
}

. "$ROOT_DIR/tests/integration/shunit2"/shunit2
