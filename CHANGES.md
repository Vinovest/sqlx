# Changes

## 1.6.0:

* Set Go version to 1.23.

* Added `AllRows` package function and `All` on statement types to
  range over rows and automatically close the result set.

* Added functions `One` and `List` for querying with
  generics. Modifications to types Stmt and NamedStmt to support
  generics may cause some small compatibility changes that should be
  quick to resolve with a search and replace. Both are now type
  aliases to attempt to minimize the impact of this change.

* fix for sql in-list parsing that will properly ignore question marks
  in comments and strings. This frequently caused a confusing error
  ("number of bindVars exceeds arguments").

## 1.5.0:

This is the first major release of sqlx from this fork in order to fix
some issues which may cause some minor compatibility changes from
previous versions, depending on how you use sqlx.

* VALUES bulk insertions were performed by a regex previously, now
  sqlx will use a tokenizer to parse the query and reliably expand the
  query for additional bound parameters. This fixes several bugs
  around named parameters (e.g. `:name`). Where previously a colon had
  to be escaped using an additional colon (resulting in `::name`), now
  the query will work as expected.

* Rebind parsing uses the tokenizer now as well, fixing double quoting
  and unexpected behavior with colons in sql strings.

* Get and GetContext will now error if the query returns more than one
  row. Previously, it fetched the first row and ignored the
  rest. Using Get on multiple rows is typically a mistake, and this
  change makes the behavior more consistent and avoids potential bugs.

* Fix for multiple sql statements ("select a from test; select b from
  test2;") in a single call. Previously, sqlx cached the columns of
  the first query and used them for all subsequent queries. This could
  lead to incorrect results if the queries returned different
  columns. Now, sqlx will reset the columns for each query on
  NextResultSet.

* Adds Queryable interface to unify DB and Tx.

These fixes are the results of upstream contributions submitted in PRs
to the original sqlx repository. Much thanks goes to everyone who
contributed their time and effort to make sqlx better. These fixes
vastly improve the usability of sqlx.

## 1.3.0:

* `sqlx.DB.Connx(context.Context) *sqlx.Conn`
* `sqlx.BindDriver(driverName, bindType)`
* support for `[]map[string]interface{}` to do "batch" insertions
* allocation & perf improvements for `sqlx.In`

DB.Connx returns an `sqlx.Conn`, which is an `sql.Conn`-alike consistent with
sqlx's wrapping of other types.

`BindDriver` allows users to control the bindvars that sqlx will use for drivers,
and add new drivers at runtime.  This results in a very slight performance hit
when resolving the driver into a bind type (~40ns per call), but it allows users
to specify what bindtype their driver uses even when sqlx has not been updated
to know about it by default.
