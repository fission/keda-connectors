#!/bin/bash
for dir in $(find . -name version); do
    connector=$(dirname $dir)
    echo "Checking $connector"
    echo "========================"
    pushd $connector
    go mod verify
    go mod download
    STATUS=0
    assert-nothing-changed() {
        local diff
        "$@" >/dev/null || return 1
        if ! diff="$(git diff -U1 --color --exit-code)"; then
            printf '\e[31mError: running `\e[1m%s\e[22m` results in modifications that you must check into version control:\e[0m\n%s\n\n' "$*" "$diff" >&2
            git checkout -- .
            STATUS=1
        fi
    }
    assert-nothing-changed go fmt ./...
    assert-nothing-changed go mod tidy
    popd
    golangci-lint run --out-format=github-actions --timeout=5m $connector || STATUS=$?
    echo "Status: $STATUS"
    echo "========================"
done
