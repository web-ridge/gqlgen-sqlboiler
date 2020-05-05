module github.com/web-ridge/gqlgen-sqlboiler/v2

go 1.14

// https://github.com/volatiletech/sqlboiler/issues/607
replace github.com/ericlagergren/decimal => github.com/ericlagergren/decimal v0.0.0-20181231230500-73749d4874d5

require (
	github.com/99designs/gqlgen v0.11.3
	github.com/gertd/go-pluralize v0.1.4
	github.com/iancoleman/strcase v0.0.0-20191112232945-16388991a334
	github.com/pkg/errors v0.9.1
	github.com/vektah/gqlparser/v2 v2.0.1
	golang.org/x/tools v0.0.0-20200501205727-542909fd9944
)
