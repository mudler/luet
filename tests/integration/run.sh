#!/bin/bash
set -e

export ROOT_DIR="$(git rev-parse --show-toplevel)"

pushd $ROOT_DIR
    go build -o "$ROOT_DIR/tests/integration/bin/luet" 
popd

[ ! -d "$ROOT_DIR/tests/integration/shunit2" ] && git clone https://github.com/kward/shunit2.git "$ROOT_DIR/tests/integration/shunit2" 

export PATH=$ROOT_DIR/tests/integration/bin/:$PATH

"$ROOT_DIR/tests/integration/01_simple.sh"
"$ROOT_DIR/tests/integration/01_simple_gzip.sh"

