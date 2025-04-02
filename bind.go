package sqlx

import (
	"database/sql/driver"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/muir/sqltoken"

	"github.com/vinovest/sqlx/reflectx"
)

// Bindvar types supported by Rebind, BindMap and BindStruct.
const (
	UNKNOWN = iota
	QUESTION
	DOLLAR
	NAMED
	AT
)

var defaultBinds = map[int][]string{
	DOLLAR:   {"postgres", "pgx", "pq-timeouts", "cloudsqlpostgres", "ql", "nrpostgres", "cockroach"},
	QUESTION: {"mysql", "sqlite3", "nrmysql", "nrsqlite3"},
	NAMED:    {"oci8", "ora", "goracle", "godror"},
	AT:       {"sqlserver", "azuresql"},
}

var binds sync.Map

var rebindConfigs = func() []sqltoken.Config {
	configs := make([]sqltoken.Config, AT+1)
	pg := sqltoken.PostgreSQLConfig()
	pg.NoticeQuestionMark = true
	pg.NoticeDollarNumber = false
	pg.SeparatePunctuation = true
	configs[DOLLAR] = pg

	ora := sqltoken.OracleConfig()
	ora.NoticeColonWord = false
	ora.NoticeQuestionMark = true
	ora.SeparatePunctuation = true
	configs[NAMED] = ora

	ssvr := sqltoken.SQLServerConfig()
	ssvr.NoticeAtWord = false
	ssvr.NoticeQuestionMark = true
	ssvr.SeparatePunctuation = true
	configs[AT] = ssvr
	return configs
}()

func init() {
	for bind, drivers := range defaultBinds {
		for _, driver := range drivers {
			BindDriver(driver, bind)
		}
	}
}

// BindType returns the bindtype for a given database given a drivername.
func BindType(driverName string) int {
	itype, ok := binds.Load(driverName)
	if !ok {
		return UNKNOWN
	}
	return itype.(int)
}

// BindDriver sets the BindType for driverName to bindType.
func BindDriver(driverName string, bindType int) {
	binds.Store(driverName, bindType)
}

// Rebind a query from the default bindtype (QUESTION) to the target bindtype.
func Rebind(bindType int, query string) string {
	switch bindType {
	case QUESTION, UNKNOWN:
		return query
	}
	config := rebindConfigs[bindType]
	tokens := sqltoken.Tokenize(query, config)
	rqb := make([]byte, 0, len(query)+10)

	var j int
	for _, token := range tokens {
		if token.Type != sqltoken.QuestionMark {
			rqb = append(rqb, ([]byte)(token.Text)...)
			continue
		}
		switch bindType {
		case DOLLAR:
			rqb = append(rqb, '$')
		case NAMED:
			rqb = append(rqb, ':', 'a', 'r', 'g')
		case AT:
			rqb = append(rqb, '@', 'p')
		}
		j++
		rqb = strconv.AppendInt(rqb, int64(j), 10)
	}
	return string(rqb)
}

func asSliceForIn(i interface{}) (v reflect.Value, ok bool) {
	if i == nil {
		return reflect.Value{}, false
	}

	v = reflect.ValueOf(i)
	t := reflectx.Deref(v.Type())

	// Only expand slices
	if t.Kind() != reflect.Slice {
		return reflect.Value{}, false
	}

	// []byte is a driver.Value type so it should not be expanded
	if t == reflect.TypeOf([]byte{}) {
		return reflect.Value{}, false
	}

	return v, true
}

// In expands slice values in args, returning the modified query string
// and a new arg list that can be executed by a database. The `query` should
// use the `?` bindVar.  The return value uses the `?` bindVar.
func In(query string, args ...interface{}) (string, []interface{}, error) {
	// argMeta stores reflect.Value and length for slices and
	// the value itself for non-slice arguments
	type argMeta struct {
		v      reflect.Value
		i      interface{}
		length int
	}

	var flatArgsCount int
	var anySlices bool

	var stackMeta [32]argMeta

	var meta []argMeta
	if len(args) <= len(stackMeta) {
		meta = stackMeta[:len(args)]
	} else {
		meta = make([]argMeta, len(args))
	}

	for i, arg := range args {
		if a, ok := arg.(driver.Valuer); ok {
			var err error
			arg, err = callValuerValue(a)
			if err != nil {
				return "", nil, err
			}
		}

		if v, ok := asSliceForIn(arg); ok {
			meta[i].length = v.Len()
			meta[i].v = v

			anySlices = true
			flatArgsCount += meta[i].length

			if meta[i].length == 0 {
				return "", nil, errors.New("empty slice passed to 'in' query")
			}
		} else {
			meta[i].i = arg
			flatArgsCount++
		}
	}

	// don't do any parsing if there aren't any slices;  note that this means
	// some errors that we might have caught below will not be returned.
	if !anySlices {
		return query, args, nil
	}

	newArgs := make([]interface{}, 0, flatArgsCount)

	var buf strings.Builder
	buf.Grow(len(query) + len(", ?")*flatArgsCount)

	var arg int
	config := rebindConfigs[DOLLAR] // specific config doesn't matter, we just need the tokenizer to return QuestionMarks
	tokens := sqltoken.Tokenize(query, config)
	inIn := false // found `in (`
	for pos, token := range tokens {
		if !inIn && token.Type == sqltoken.Punctuation && token.Text == "(" {
			// look backwards to see if the previous word is "in"
			for i := pos - 1; i >= 0; i-- {
				if tokens[i].Type == sqltoken.Word {
					inIn = strings.ToLower(tokens[i].Text) == "in"
					break
				}
			}
		}
		if token.Type == sqltoken.QuestionMark {
			if arg >= len(meta) {
				// if an argument wasn't passed, lets return an error;  this is
				// not actually how database/sql Exec/Query works, but since we are
				// creating an argument list programmatically, we want to be able
				// to catch these programmer errors earlier.
				return "", nil, errors.New("number of bindVars exceeds arguments")
			}

			argMeta := meta[arg]
			arg++

			// not an in-list
			if !inIn {
				newArgs = append(newArgs, argMeta.i)
				buf.WriteString(token.Text)
				continue
			}

			buf.WriteString("?")
			for si := 1; si < argMeta.length; si++ {
				buf.WriteString(", ?")
			}

			newArgs = appendReflectSlice(newArgs, argMeta.v, argMeta.length)
		} else if inIn && token.Type == sqltoken.Punctuation && token.Text == ")" {
			inIn = false
			buf.WriteString(token.Text)
		} else {
			buf.WriteString(token.Text)
		}
	}

	if arg < len(meta) {
		return "", nil, errors.New("number of bindVars less than number arguments")
	}

	return buf.String(), newArgs, nil
}

func appendReflectSlice(args []interface{}, v reflect.Value, vlen int) []interface{} {
	switch val := v.Interface().(type) {
	case []interface{}:
		args = append(args, val...)
	case []int:
		for i := range val {
			args = append(args, val[i])
		}
	case []string:
		for i := range val {
			args = append(args, val[i])
		}
	default:
		for si := 0; si < vlen; si++ {
			args = append(args, v.Index(si).Interface())
		}
	}

	return args
}

// callValuerValue returns vr.Value(), with one exception:
// If vr.Value is an auto-generated method on a pointer type and the
// pointer is nil, it would panic at runtime in the panicwrap
// method. Treat it like nil instead.
// Issue 8415.
//
// This is so people can implement driver.Value on value types and
// still use nil pointers to those types to mean nil/NULL, just like
// string/*string.
//
// This function is copied from database/sql/driver package
// and mirrored in the database/sql package.
func callValuerValue(vr driver.Valuer) (v driver.Value, err error) {
	if rv := reflect.ValueOf(vr); rv.Kind() == reflect.Pointer &&
		rv.IsNil() &&
		rv.Type().Elem().Implements(reflect.TypeOf((*driver.Valuer)(nil)).Elem()) {
		return nil, nil
	}
	return vr.Value()
}
