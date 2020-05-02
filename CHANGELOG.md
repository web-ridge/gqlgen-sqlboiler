# Changelog

## May 2

A few breaking changes, after this we will try to keep everything stable.
We needed this since the go modules did not work well with the current approach.

- Remove self-referencing fixes for converts (https://github.com/volatiletech/sqlboiler/issues/522)
- Upgrading to sqlboiler v4.0.0 (https://github.com/volatiletech/sqlboiler/releases)
- Move examples to https://github.com/web-ridge/gqlgen-sqlboiler/v2-examples
- Move convert utils to https://github.com/web-ridge/utils-go
- Going to one go modules for the repository

## April 1-6

- Better relationships support
- Better helpers for reading the boiler structs
- Require uints for id's and removing other code
- Difference between create/update inputs support
- Fix for preloads where it sometimes was missing things
