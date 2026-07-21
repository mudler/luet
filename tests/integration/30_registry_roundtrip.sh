#!/bin/bash

export LUET_NOLOCK=true

# Exercises a full build -> create-repo -> push -> pull -> install cycle
# against a local registry. This is a regression guard for the Docker 29
# push shape: newer daemons may push single-platform images as OCI indexes
# rather than plain manifests (moby/moby#51532), and the daemonless pull
# side must cope with whichever shape it gets. On Docker 29.1.2 the push was
# observed as a plain OCI manifest, not an index -- the shape varies by
# daemon version, so the test logs it rather than asserting on it.
# The quay-backed docker tests skip without credentials, so this local
# registry roundtrip is what actually exercises the path in CI.

REGISTRY_PORT=${REGISTRY_PORT:-5000}
REGISTRY_NAME=luet-test-registry
LOCAL_IMAGE="localhost:${REGISTRY_PORT}/luet-roundtrip"

oneTimeSetUp() {
    export tmpdir="$(mktemp -d)"
    docker images --filter='reference=luet/cache' --format='{{.Repository}}:{{.Tag}}' | xargs -r docker rmi
    docker rm -f "$REGISTRY_NAME" >/dev/null 2>&1
    registryout=$(docker run -d --name "$REGISTRY_NAME" \
        -p "${REGISTRY_PORT}:5000" registry:2 2>&1)
    registryst=$?
    if [ "$registryst" -ne 0 ]; then
        echo "failed to start registry container on port ${REGISTRY_PORT}: $registryout" >&2
    fi
    assertEquals "started the local registry container: $registryout" "0" "$registryst"

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
    docker images --filter='reference=luet/cache' --format='{{.Repository}}:{{.Tag}}' | xargs -r docker rmi
}

testRegistryUp() {
    [ "$LUET_BACKEND" == "img" ] && startSkipping
    curl -sf "http://localhost:${REGISTRY_PORT}/v2/" >/dev/null
    assertEquals 'local registry is reachable' "0" "$?"
}

testBuild() {
    [ "$LUET_BACKEND" == "img" ] && startSkipping
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
    [ "$LUET_BACKEND" == "img" ] && startSkipping
    createres=$(luet create-repo --tree "$ROOT_DIR/tests/fixtures/docker_repo" \
        --output "${LOCAL_IMAGE}" \
        --packages "$tmpdir/testbuild" \
        --name "test" \
        --descr "Test Repo" \
        --urls "$tmpdir/testrootfs" \
        --tree-compression zstd \
        --tree-filename foo.tar \
        --meta-filename repository.meta.tar \
        --meta-compression zstd \
        --type docker --push-images --force-push)
    createst=$?

    echo "$createres"

    assertEquals 'pushes repository to local registry' "0" "$createst"
    assertContains 'contains image push' "$createres" 'Pushed image:'

    # Log-only: which manifest shape the daemon actually produced. Docker
    # versions differ here (plain manifest vs OCI index), so this is recorded
    # for CI diagnosis and deliberately not asserted on.
    curl -sf -H 'Accept: application/vnd.oci.image.index.v1+json, application/vnd.oci.image.manifest.v1+json' \
         -o /dev/null -w 'pushed manifest content-type: %{content_type}\n' \
         "http://localhost:${REGISTRY_PORT}/v2/luet-roundtrip/manifests/repository.yaml" || true
}

# The pull side is daemonless (go-containerregistry remote), so this is
# where an unexpected OCI index shape surfaces.
testInstallFromRegistry() {
    [ "$LUET_BACKEND" == "img" ] && startSkipping
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
