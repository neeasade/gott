#!/usr/bin/env bash
# sanity integration test

if ! go build; then
   exit 1
fi

toml=$(cat <<EOF
ref = '{{.b.localA}}'
# ref1 = '{{.a.dashed-ident}}'
ref2 = '{{.a.zero}}'
negative-one = '{{sub .a.zero 1}}'

[a]
a = 'wow!!'
a2 = 'ok {{.a}}'
a3 = [ 'fancy', 'words' ]
# a4 = '{{.a3.0}}'
dashed-ident = '{{.a}}'
one = 1
zero = '{{sub .one 1}}'

[b]
a = '{{.a.a}}'
b = 'b'
localA = '{{.a}}'
c = '{{.a}} {{.b}}'
EOF
)

expected=$(cat <<EOF
negative-one = '-1'
ref = 'wow!!'
ref1 = 'wow!!'
ref2 = '0'
[a]
a = 'wow!!'
a2 = 'ok wow!!'
a3 = ['fancy', 'words']
a4 = 'fancy'
dashed-ident = 'wow!!'
one = 1
zero = '0'

[b]
a = 'wow!!'
b = 'b'
c = 'wow!! b'
localA = 'wow!!'
EOF
)

die() {
    printf "$1\n" >&2
    exit 1
}

if [ "$1" = "-d" ]; then
    # debug
    # timeout .1 ./gott -T "$toml" -v -o toml
    timeout 1 ./gott -T "$toml" -v -o toml
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
    diff -y <(echo "$expected") <(echo "$result")
fi
