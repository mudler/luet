#!/bin/bash
set -e
export LUET_NO_SPINNER=true
export LUET_YES=true
export ROOT_DIR="$(git rev-parse --show-toplevel)"

pushd $ROOT_DIR
    go build -o "$ROOT_DIR/tests/integration/bin/luet" 
popd

[ ! -d "$ROOT_DIR/tests/integration/shunit2" ] && git clone https://github.com/kward/shunit2.git "$ROOT_DIR/tests/integration/shunit2" 

export PATH=$ROOT_DIR/tests/integration/bin/:$PATH

if [ -z "$SINGLE_TEST" ]; then

  for script in $(ls "$ROOT_DIR/tests/integration/" | grep '^[0-9]*_.*.sh'); do
    echo "Executing test '$script'."
    $ROOT_DIR/tests/integration/$script
  done

else

  echo "Executing test '$SINGLE_TEST'."
  $ROOT_DIR/tests/integration/$SINGLE_TEST

fi