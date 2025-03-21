package sqlx

// Named Query Support
//
//  * BindMap - bind query bindvars to map/struct args
//	* NamedExec, NamedQuery - named query w/ struct or map
//  * NamedStmt - a pre-compiled named query which is a prepared statement
//
// Internal Interfaces:
//
//  * compileNamedQuery - rebind a named query, returning a query and list of names
//  * bindArgs, bindMapArgs, bindAnyArgs - given a list of names, return an arglist
//
import (
	"bytes"
	"database/sql"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/muir/sqltoken"

	"github.com/vinovest/sqlx/reflectx"
)

// NamedStmt is a prepared statement that executes named queries.  Prepare it
// how you would execute a NamedQuery, but pass in a struct or map when executing.
type NamedStmt struct {
	Params      []string
	QueryString string
	Stmt        *Stmt
}

// Close closes the named statement.
func (n *NamedStmt) Close() error {
	return n.Stmt.Close()
}

// Exec executes a named statement using the struct passed.
// Any named placeholder parameters are replaced with fields from arg.
func (n *NamedStmt) Exec(arg interface{}) (sql.Result, error) {
	args, err := bindAnyArgs(n.Params, arg, n.Stmt.Mapper)
	if err != nil {
		return *new(sql.Result), err
	}
	return n.Stmt.Exec(args...)
}

// Query executes a named statement using the struct argument, returning rows.
// Any named placeholder parameters are replaced with fields from arg.
func (n *NamedStmt) Query(arg interface{}) (*sql.Rows, error) {
	args, err := bindAnyArgs(n.Params, arg, n.Stmt.Mapper)
	if err != nil {
		return nil, err
	}
	return n.Stmt.Query(args...)
}

// QueryRow executes a named statement against the database.  Because sqlx cannot
// create a *sql.Row with an error condition pre-set for binding errors, sqlx
// returns a *sqlx.Row instead.
// Any named placeholder parameters are replaced with fields from arg.
func (n *NamedStmt) QueryRow(arg interface{}) *Row {
	args, err := bindAnyArgs(n.Params, arg, n.Stmt.Mapper)
	if err != nil {
		return &Row{err: err}
	}
	return n.Stmt.QueryRowx(args...)
}

// MustExec execs a NamedStmt, panicing on error
// Any named placeholder parameters are replaced with fields from arg.
func (n *NamedStmt) MustExec(arg interface{}) sql.Result {
	res, err := n.Exec(arg)
	if err != nil {
		panic(err)
	}
	return res
}

// Queryx using this NamedStmt
// Any named placeholder parameters are replaced with fields from arg.
func (n *NamedStmt) Queryx(arg interface{}) (*Rows, error) {
	r, err := n.Query(arg)
	if err != nil {
		return nil, err
	}
	return &Rows{Rows: r, Mapper: n.Stmt.Mapper, unsafe: isUnsafe(n)}, err
}

// QueryRowx this NamedStmt.  Because of limitations with QueryRow, this is
// an alias for QueryRow.
// Any named placeholder parameters are replaced with fields from arg.
func (n *NamedStmt) QueryRowx(arg interface{}) *Row {
	return n.QueryRow(arg)
}

// Select using this NamedStmt
// Any named placeholder parameters are replaced with fields from arg.
func (n *NamedStmt) Select(dest interface{}, arg interface{}) error {
	rows, err := n.Queryx(arg)
	if err != nil {
		return err
	}
	// if something happens here, we want to make sure the rows are Closed
	defer rows.Close()
	return scanAll(rows, dest, false)
}

// Get using this NamedStmt
// Any named placeholder parameters are replaced with fields from arg.
func (n *NamedStmt) Get(dest interface{}, arg interface{}) error {
	r := n.QueryRowx(arg)
	return r.scanAny(dest, false)
}

// Unsafe creates an unsafe version of the NamedStmt
func (n *NamedStmt) Unsafe() *NamedStmt {
	r := &NamedStmt{Params: n.Params, Stmt: n.Stmt, QueryString: n.QueryString}
	r.Stmt.unsafe = true
	return r
}

// A union interface of preparer and binder, required to be able to prepare
// named statements (as the bindtype must be determined).
type namedPreparer interface {
	Preparer
	binder
}

func prepareNamed(p namedPreparer, query string) (*NamedStmt, error) {
	bindType := BindType(p.DriverName())
	compiled, err := compileNamedQuery([]byte(query), bindType)
	if err != nil {
		return nil, err
	}
	stmt, err := Preparex(p, compiled.query)
	if err != nil {
		return nil, err
	}
	return &NamedStmt{
		QueryString: compiled.query,
		Params:      compiled.names,
		Stmt:        stmt,
	}, nil
}

// convertMapStringInterface attempts to convert v to map[string]interface{}.
// Unlike v.(map[string]interface{}), this function works on named types that
// are convertible to map[string]interface{} as well.
func convertMapStringInterface(v interface{}) (map[string]interface{}, bool) {
	var m map[string]interface{}
	mtype := reflect.TypeOf(m)
	t := reflect.TypeOf(v)
	if !t.ConvertibleTo(mtype) {
		return nil, false
	}
	return reflect.ValueOf(v).Convert(mtype).Interface().(map[string]interface{}), true
}

func bindAnyArgs(names []string, arg interface{}, m *reflectx.Mapper) ([]interface{}, error) {
	if maparg, ok := convertMapStringInterface(arg); ok {
		return bindMapArgs(names, maparg)
	}
	return bindArgs(names, arg, m)
}

// private interface to generate a list of interfaces from a given struct
// type, given a list of names to pull out of the struct.  Used by public
// BindStruct interface.
func bindArgs(names []string, arg interface{}, m *reflectx.Mapper) ([]interface{}, error) {
	arglist := make([]interface{}, 0, len(names))

	// grab the indirected value of arg
	var v reflect.Value
	for v = reflect.ValueOf(arg); v.Kind() == reflect.Ptr; {
		v = v.Elem()
	}

	err := m.TraversalsByNameFunc(v.Type(), names, func(i int, t []int) error {
		if len(t) == 0 {
			return fmt.Errorf("could not find name %s in %#v", names[i], arg)
		}

		val := reflectx.FieldByIndexesReadOnly(v, t)
		arglist = append(arglist, val.Interface())

		return nil
	})

	return arglist, err
}

// like bindArgs, but for maps.
func bindMapArgs(names []string, arg map[string]interface{}) ([]interface{}, error) {
	arglist := make([]interface{}, 0, len(names))

	for _, name := range names {
		val, ok := arg[name]
		if !ok {
			return arglist, fmt.Errorf("could not find name %s in %#v", name, arg)
		}
		arglist = append(arglist, val)
	}
	return arglist, nil
}

// bindStruct binds a named parameter query with fields from a struct argument.
// The rules for binding field names to parameter names follow the same
// conventions as for StructScan, including obeying the `db` struct tags.
func bindStruct(bindType int, query string, arg interface{}, m *reflectx.Mapper) (string, []interface{}, error) {
	compiled, err := compileNamedQuery([]byte(query), bindType)
	if err != nil {
		return "", []interface{}{}, err
	}

	arglist, err := bindAnyArgs(compiled.names, arg, m)
	if err != nil {
		return "", []interface{}{}, err
	}

	return compiled.query, arglist, nil
}

func fixBound(cq *compiledQueryResult, loop int) {
	if cq.valuesStart == nil || cq.valuesEnd == nil {
		return
	}
	length := (int(*cq.valuesEnd-1)-int(*cq.valuesStart))*loop +
		// bytes for commas, too
		(loop - 1)
	buffer := bytes.NewBuffer(make([]byte, 0, length))

	buffer.WriteString(cq.query[0:*cq.valuesEnd])
	for i := 0; i < loop-1; i++ {
		buffer.WriteString(",")
		buffer.WriteString(cq.query[*cq.valuesStart:*cq.valuesEnd])
	}
	buffer.WriteString(cq.query[*cq.valuesEnd:])
	cq.query = buffer.String()
}

// bindArray binds a named parameter query with fields from an array or slice of
// structs argument.
func bindArray(bindType int, query string, arg interface{}, m *reflectx.Mapper) (string, []interface{}, error) {
	// do the initial binding with QUESTION;  if bindType is not question,
	// we can rebind it at the end.
	compiled, err := compileNamedQuery([]byte(query), QUESTION)
	if err != nil {
		return "", []interface{}{}, err
	}
	arrayValue := reflect.ValueOf(arg)
	arrayLen := arrayValue.Len()
	if arrayLen == 0 {
		return "", []interface{}{}, fmt.Errorf("length of array is 0: %#v", arg)
	}
	arglist := make([]interface{}, 0, len(compiled.names)*arrayLen)
	for i := 0; i < arrayLen; i++ {
		elemArglist, err := bindAnyArgs(compiled.names, arrayValue.Index(i).Interface(), m)
		if err != nil {
			return "", []interface{}{}, err
		}
		arglist = append(arglist, elemArglist...)
	}
	if arrayLen > 1 {
		fixBound(&compiled, arrayLen)
	}
	// adjust binding type if we weren't on question
	bound := compiled.query
	if bindType != QUESTION {
		bound = Rebind(bindType, bound)
	}
	return bound, arglist, nil
}

// bindMap binds a named parameter query with a map of arguments.
func bindMap(bindType int, query string, args map[string]interface{}) (string, []interface{}, error) {
	compiled, err := compileNamedQuery([]byte(query), bindType)
	if err != nil {
		return "", []interface{}{}, err
	}

	arglist, err := bindMapArgs(compiled.names, args)
	return compiled.query, arglist, err
}

var namedParseConfigs = func() []sqltoken.Config {
	configs := make([]sqltoken.Config, AT+1)
	pg := sqltoken.PostgreSQLConfig()
	pg.NoticeColonWord = true
	pg.ColonWordIncludesUnicode = true
	pg.NoticeDollarNumber = false
	pg.NoticeQuestionMark = true
	configs[DOLLAR] = pg

	ora := sqltoken.OracleConfig()
	ora.ColonWordIncludesUnicode = true
	ora.NoticeQuestionMark = true
	configs[NAMED] = ora

	ssvr := sqltoken.SQLServerConfig()
	ssvr.NoticeColonWord = true
	ssvr.ColonWordIncludesUnicode = true
	ssvr.NoticeAtWord = false
	configs[AT] = ssvr

	mysql := sqltoken.MySQLConfig()
	mysql.NoticeColonWord = true
	mysql.ColonWordIncludesUnicode = true
	mysql.NoticeQuestionMark = true
	configs[QUESTION] = mysql
	configs[UNKNOWN] = mysql
	return configs
}()

// -- Compilation of Named Queries

// FIXME: this function isn't safe for unicode named params, as a failing test
// can testify.  This is not a regression but a failure of the original code
// as well.  It should be modified to range over runes in a string rather than
// bytes, even though this is less convenient and slower.  Hopefully the
// addition of the prepared NamedStmt (which will only do this once) will make
// up for the slightly slower ad-hoc NamedExec/NamedQuery.

type compiledQueryResult struct {
	// the query string with the named parameters swapped out with bindvars
	query string

	// the name of the parameter
	names []string

	// byte position in the query string, start of it including :
	namePositions []uint32

	// if set, the start position of the VALUES argument list not including () (end inclusive)
	valuesStart *uint32
	valuesEnd   *uint32
}

// compile a NamedQuery into an unbound query (using the '?' bindvar) and
// a list of names.
func compileNamedQuery(qs []byte, bindType int) (compiledQueryResult, error) {
	r := compiledQueryResult{
		names:         make([]string, 0, 10),
		namePositions: make([]uint32, 0, 10),
	}
	curpos := uint32(0)
	rebound := make([]byte, 0, len(qs))
	inValues := false
	inValuesOpenCount := 0

	currentVar := 1
	tokens := sqltoken.Tokenize(string(qs), namedParseConfigs[bindType])

	for _, token := range tokens {
		if token.Type == sqltoken.Word && strings.EqualFold("values", token.Text) && !inValues {
			// current behavior: expand the first values and ignore the rest
			if r.valuesStart == nil { // did we already parse a values statement?
				inValues = true
			}
		}
		if inValues && token.Type == sqltoken.Punctuation {
			if token.Text == "(" {
				start := curpos
				inValuesOpenCount += 1
				if r.valuesStart == nil {
					r.valuesStart = &start
				}
			}
			if token.Text == ")" {
				inValuesOpenCount -= 1
				if inValuesOpenCount == 0 {
					end := curpos + 1
					r.valuesEnd = &end
					inValues = false
				}
			}
		}
		if token.Type != sqltoken.ColonWord {
			rebound = append(rebound, ([]byte)(token.Text)...)
			curpos += uint32(len(token.Text))
			continue
		}
		r.names = append(r.names, token.Text[1:])
		r.namePositions = append(r.namePositions, curpos)

		newBound := ""
		switch bindType {
		// oracle only supports named type bind vars even for positional
		case NAMED:
			newBound = token.Text
		case QUESTION, UNKNOWN:
			newBound = "?"
		case DOLLAR:
			newBound = "$" + strconv.Itoa(currentVar)
			currentVar++
		case AT:
			newBound = "@p" + strconv.Itoa(currentVar)
			currentVar++
		}

		rebound = append(rebound, []byte(newBound)...)
		curpos += uint32(len(newBound))
	}

	if inValues {
		return r, fmt.Errorf("missing closing bracket in VALUES")
	}
	r.query = string(rebound)
	return r, nil
}

// BindNamed binds a struct or a map to a query with named parameters.
// DEPRECATED: use sqlx.Named` instead of this, it may be removed in future.
func BindNamed(bindType int, query string, arg interface{}) (string, []interface{}, error) {
	return bindNamedMapper(bindType, query, arg, mapper())
}

// Named takes a query using named parameters and an argument and
// returns a new query with a list of args that can be executed by
// a database.  The return value uses the `?` bindvar.
func Named(query string, arg interface{}) (string, []interface{}, error) {
	return bindNamedMapper(QUESTION, query, arg, mapper())
}

func bindNamedMapper(bindType int, query string, arg interface{}, m *reflectx.Mapper) (string, []interface{}, error) {
	t := reflect.TypeOf(arg)
	k := t.Kind()
	switch {
	case k == reflect.Map && t.Key().Kind() == reflect.String:
		m, ok := convertMapStringInterface(arg)
		if !ok {
			return "", nil, fmt.Errorf("sqlx.bindNamedMapper: unsupported map type: %T", arg)
		}
		return bindMap(bindType, query, m)
	case k == reflect.Array || k == reflect.Slice:
		return bindArray(bindType, query, arg, m)
	default:
		return bindStruct(bindType, query, arg, m)
	}
}

// NamedQuery binds a named query and then runs Query on the result using the
// provided Ext (sqlx.Tx, sqlx.Db).  It works with both structs and with
// map[string]interface{} types.
func NamedQuery(e Ext, query string, arg interface{}) (*Rows, error) {
	q, args, err := bindNamedMapper(BindType(e.DriverName()), query, arg, mapperFor(e))
	if err != nil {
		return nil, err
	}
	return e.Queryx(q, args...)
}

// NamedExec uses BindStruct to get a query executable by the driver and
// then runs Exec on the result.  Returns an error from the binding
// or the query execution itself.
func NamedExec(e Ext, query string, arg interface{}) (sql.Result, error) {
	q, args, err := bindNamedMapper(BindType(e.DriverName()), query, arg, mapperFor(e))
	if err != nil {
		return nil, err
	}
	return e.Exec(q, args...)
}
