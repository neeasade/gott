#!/usr/bin/env bash
# sanity integration test

toml=$(cat <<EOF
ok = '{{.b.localA}}'
[a]
a = 'wow!!'
a2 = 'ok {{.a}}'
one = 1
zero = '{{sub .one 1}}'

[b]
a = '{{.a.a}}'
b = 'b'
localA = '{{.a}}'
EOF
)

expected=$(cat <<EOF
ok = 'wow!!'
[a]
a = 'wow!!'
a2 = 'ok wow!!'
one = 1
zero = '0'

[b]
a = 'wow!!'
b = 'b'
localA = 'wow!!'
EOF
)

die() {
    printf "$1\n" >&2
    exit 1
}

if [ "$1" = "-d" ]; then
    # debug
    timeout .1 ./gott -T "$toml" -v -o toml
    exit $?
fi

result=$(timeout 1 ./gott -T "$toml" -o toml)
case $? in
    0) ;;
    124) die "render timed out.";;
    *) die "render failed." ;;
esac

if ! test "$expected" = "$result"; then
    echo "test failed!"
    echo "left: expected, right: result"
    diff -w -y <(echo "$expected") <(echo "$result")
fi
