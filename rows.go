package rows

import (
	"fmt"
	"reflect"
)

type Rows interface {
	Next() bool
	Columns() ([]string, error)
	Scan(dest ...interface{}) error
	Err() error
}

func RowsScan(rows Rows, v interface{}, fn func(reflect.StructField) string) (int, error) {
	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Ptr {
		return 0, ErrNotPointer
	}
	val = val.Elem()
	for val.Kind() == reflect.Ptr {
		if val.IsNil() {
			val.Set(reflect.New(val.Type().Elem()))
		}
		val = val.Elem()
	}

	limit := 0
	switch val.Kind() {
	case reflect.Array:
		limit = val.Len()
	case reflect.Slice:
		limit = -1
	default:
		limit = 1
	}
	key, data, err := RowsLimitChannel(rows, limit)
	if err != nil {
		return 0, err
	}

	err = DataScanChannel(key, data, v, fn)
	if err != nil {
		return 0, err
	}
	return len(data), nil
}

// rowsLimit
func rowsLimit(rows Rows, limit int, g bool, df func(d [][]byte)) ([]string, error) {
	if limit == 0 {
		return nil, nil
	}
	if !rows.Next() {
		return nil, nil
	}
	key, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	keysize := len(key)
	if keysize == 0 {
		return nil, nil
	}

	ff := func() {
		for i := 0; i != limit; i++ {
			r := makeBytesInterface(keysize)
			if err := rows.Scan(r...); err != nil {
				fmt.Sprintln(err)
				break
			}

			df(rowsInterfaceToByte(r))
			if !rows.Next() {
				break
			}
		}
		df(nil)
	}
	if g {
		go ff()
	} else {
		ff()
	}

	return key, nil
}

// rowScanSlice row scan Slice
func rowScanSlice(key []string, d [][]byte, val reflect.Value) error {
	switch val.Kind() {
	default:
		return ErrInvalidType
	case reflect.Ptr:
		return rowScanSlice(key, d, val.Elem())
	case reflect.Slice:
		return rowScanSliceValue(key, d, val)
	}
	return nil
}

// rowScanSliceValue row scan Slice value
func rowScanSliceValue(key []string, d [][]byte, val reflect.Value) error {
	tt := val.Type()
	te := tt.Elem()
	switch te.Kind() {
	default:
		return ErrInvalidType
	case reflect.String:
		l := len(d)
		val.Set(reflect.MakeSlice(tt, l, l))
		for k, _ := range key {
			val.Index(k).Set(reflect.ValueOf(string(d[k])))
		}
	case reflect.Slice:
		if te.Elem().Kind() != reflect.Uint8 {
			return ErrInvalidType
		}
		val.Set(reflect.ValueOf(d))
	}
	return nil

}

// rowScanMap row scan Map
func rowScanMap(key []string, d [][]byte, val reflect.Value) error {
	switch val.Kind() {
	default:
		return ErrInvalidType
	case reflect.Ptr:
		return rowScanMap(key, d, val.Elem())
	case reflect.Map:
		return rowScanMapValue(key, d, val)
	}
	return nil
}

// rowScanMapValue row scan Map value
func rowScanMapValue(key []string, d [][]byte, val reflect.Value) error {
	tt := val.Type()
	if tt.Key().Kind() != reflect.String {
		return ErrInvalidType
	}
	val.Set(reflect.MakeMap(tt))
	te := tt.Elem()
	switch te.Kind() {
	default:
		return ErrInvalidType
	case reflect.String:
		for k, v := range key {
			val.SetMapIndex(reflect.ValueOf(v), reflect.ValueOf(string(d[k])))
		}
	case reflect.Slice:
		if te.Elem().Kind() != reflect.Uint8 {
			return ErrInvalidType
		}
		for k, v := range key {
			val.SetMapIndex(reflect.ValueOf(v), reflect.ValueOf(d[k]))
		}
	}
	return nil
}

// rowScanStruct row scan Struct
func rowScanStruct(key [][]string, d [][]byte, val reflect.Value) error {
	switch val.Kind() {
	default:
		return ErrInvalidType
	case reflect.Ptr:
		return rowScanStruct(key, d, val.Elem())
	case reflect.Struct:
		for k, v := range key {
			if len(v) == 0 {
				continue
			}

			fi := val
			for _, v0 := range v {
				fi = fi.FieldByName(v0)
			}
			fi = fi.Addr()
			if err := ConvertAssign(fi.Interface(), d[k]); err != nil {
				return err
			}
		}
		return nil
	}
	return nil
}

// rows2MapStrings rows to map string
func rows2MapStrings(key []string, data [][][]byte) []map[string]string {
	m := make([]map[string]string, 0, len(data))
	for _, v := range data {
		m = append(m, rows2MapString(key, v))
	}
	return m
}

// rows2MapString rows to map string
func rows2MapString(key []string, v [][]byte) map[string]string {
	m0 := map[string]string{}
	for i, k := range key {
		if vv := v[i]; len(vv) == 0 {
			m0[k] = ""
		} else {
			m0[k] = string(vv)
		}
	}
	return m0
}

// rows2Maps rows to map
func rows2Maps(key []string, data [][][]byte) []map[string][]byte {
	m := make([]map[string][]byte, 0, len(data))
	for _, v := range data {
		m = append(m, rows2Map(key, v))
	}
	return m
}

// rows2Map rows to map
func rows2Map(key []string, v [][]byte) map[string][]byte {
	m0 := map[string][]byte{}
	for i, k := range key {
		if vv := v[i]; len(vv) == 0 {
			m0[k] = []byte{}
		} else {
			m0[k] = vv
		}
	}

	return m0
}

// rows2Table rows to table
func rows2Table(key []string, data [][][]byte) [][]string {
	m := make([][]string, 0, len(data)+1)
	m = append(m, key)
	for _, v := range data {
		m0 := make([]string, 0, len(v))
		for _, v0 := range v {
			if len(v0) == 0 {
				m0 = append(m0, "")
			} else {
				m0 = append(m0, string(v0))
			}
		}
		m = append(m, m0)
	}
	return m
}

func makeBytesInterface(max int) []interface{} {
	r := make([]interface{}, 0, max)
	for i := 0; i != max; i++ {
		r = append(r, &[]byte{})
	}
	return r
}

func rowsInterfaceToByte(m []interface{}) [][]byte {
	r0 := make([][]byte, 0, len(m))
	for _, v := range m {
		if v0, ok := v.(*[]byte); ok && v0 != nil {
			r0 = append(r0, *v0)
		} else {
			r0 = append(r0, []byte{})
		}
	}
	return r0
}

func rowsScanValueFunc(tt reflect.Type, key []string, fn func(reflect.StructField) string) (func(key []string, d [][]byte, val reflect.Value) error, error) {
	switch tt.Kind() {
	default:
		return nil, ErrInvalidType
	case reflect.Struct:
		key0 := colAdjust(tt, key, fn)
		return func(key []string, d [][]byte, val reflect.Value) error {
			return rowScanStruct(key0, d, val)
		}, nil
	case reflect.Map:
		return rowScanMap, nil
	case reflect.Slice:
		return rowScanSlice, nil
	}
}
