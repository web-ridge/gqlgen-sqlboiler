# Changelog

## May 2 - v2

A few breaking changes, after this we will try to keep everything stable.
We needed this since the go modules did not work well with the current approach.

- Remove self-referencing fixes for converts (https://github.com/volatiletech/sqlboiler/issues/522)
- Upgrading to sqlboiler v4.0.0 (https://github.com/volatiletech/sqlboiler/releases)
- Move examples to https://github.com/web-ridge/gqlgen-sqlboiler-examples
- Move convert utils to https://github.com/web-ridge/utils-go
- Going to one go modules for the repository

### How to upgrade

- Change all github.com/web-ridge/gqlgen-sqlboiler/SOME_PACKAGE to github.com/web-ridge/gqlgen-sqlboiler/v2
- e.g. convert_plugin.go see changes here: https://github.com/web-ridge/gqlgen-sqlboiler-examples/commit/4ce348645380014c0b9c8700dc04ff03779366c5#diff-93204d7629b0baba6dc6614d4233e41d

Always make sure you're up to date with running:

```
go get github.com/web-ridge/gqlgen-sqlboiler/v2@LATEST_COMMIT
```

E.g.

```
go get github.com/web-ridge/gqlgen-sqlboiler/v2@46d41d9db0dff39411f528e23cff66e9eb629c39
```

## April 1-6

- Better relationships support
- Better helpers for reading the boiler structs
- Require uints for id's and removing other code
- Difference between create/update inputs support
- Fix for preloads where it sometimes was missing things
