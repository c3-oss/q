# @c3-oss/q

A read-only, multi-database query CLI.

```sh
npm install -g @c3-oss/q
echo "$CONN_STRING" | q 'select id, email from users limit 10'
```

`q` is a small JavaScript shim that delegates to the prebuilt Go binary
matching your platform, distributed through npm `optionalDependencies`.
There is no `postinstall` download — npm filters by `os` and `cpu` and
only installs the sub-package your machine needs.

Supported platforms:

- macOS arm64 / amd64
- Linux amd64 / arm64

Source code, documentation, and issue tracker:
<https://github.com/c3-oss/q>
