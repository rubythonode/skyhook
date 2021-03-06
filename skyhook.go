// Package skyhook provides an easy way to wrap skylark scripts with go
// functions.
package skyhook

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/google/skylark"
)

// Skyhook is a script/plugin runner.
type Skyhook struct {
	dirs []string
}

// New returns a Skyhook that looks in the given directories for plugin files to
// run.  The directories are searched in order for files when Run is called.
func New(dirs []string) Skyhook {
	return Skyhook{dirs}
}

// Run looks for a file with the given filename, and runs it with the given args
// passed to the script's global namespace. The return value is all convertible
// global variables from the script.
func (s Skyhook) Run(filename string, args map[string]interface{}) (map[string]interface{}, error) {
	for _, d := range s.dirs {
		b, err := ioutil.ReadFile(filepath.Join(d, filename))
		if err == nil {
			return s.exec(filename, b, args)
		}
	}
	return nil, fmt.Errorf("cannot find plugin file %q in any plugin directoy", filename)
}

func (s Skyhook) exec(filename string, data []byte, args map[string]interface{}) (map[string]interface{}, error) {
	thread := &skylark.Thread{
		Print: func(_ *skylark.Thread, msg string) { fmt.Println(msg) },
	}
	globals, err := MakeStringDict(args)
	if err != nil {
		return nil, err
	}

	if err := skylark.ExecFile(thread, filename, data, globals); err != nil {
		return nil, err
	}

	return FromStringDict(globals), nil
}

// ToValue attempts to convert the given value to a skylark.Value.  It supports
// all int, uint, and float numeric types, strings, and bools.  Any
// skylark.Value is passed through as-is.  A []interface{} is converted with
// MakeList, map[interface{}]interface{} is converted with MakeDict, and
// map[interface{}]bool is converted with MakeSet.
func ToValue(v interface{}) (skylark.Value, error) {
	if val, ok := v.(skylark.Value); ok {
		return val, nil
	}
	switch v := v.(type) {
	case int:
		return skylark.MakeInt(v), nil
	case int8:
		return skylark.MakeInt(int(v)), nil
	case int16:
		return skylark.MakeInt(int(v)), nil
	case int32:
		return skylark.MakeInt(int(v)), nil
	case int64:
		return skylark.MakeInt64(v), nil
	case uint:
		return skylark.MakeUint(v), nil
	case uint8:
		return skylark.MakeUint(uint(v)), nil
	case uint16:
		return skylark.MakeUint(uint(v)), nil
	case uint32:
		return skylark.MakeUint(uint(v)), nil
	case uint64:
		return skylark.MakeUint64(v), nil
	case bool:
		return skylark.Bool(v), nil
	case string:
		return skylark.String(v), nil
	case float32:
		return skylark.Float(float64(v)), nil
	case float64:
		return skylark.Float(v), nil
	case []interface{}:
		// There's no way to tell if they want a tuple or a list, so we default
		// to the more permissive list type.
		return MakeList(v)
	case map[interface{}]interface{}:
		// Dict
		return MakeDict(v)
	case map[interface{}]bool:
		// Set
		return MakeSet(v)
	}

	return nil, fmt.Errorf("type %T is not a supported skylark type", v)
}

// FromValue converts a skylark value to a go value.
func FromValue(v skylark.Value) (interface{}, error) {
	switch v := v.(type) {
	case skylark.Bool:
		return bool(v), nil
	case skylark.Int:
		if i, ok := v.Int64(); ok {
			return i, nil
		}
		if i, ok := v.Uint64(); ok {
			return i, nil
		}
		// buh... maybe > maxint64?  Dunno
		return nil, fmt.Errorf("can't convert skylark.Int %q to int", v)
	case skylark.Float:
		return float64(v), nil
	case skylark.String:
		return string(v), nil
	case *skylark.List:
		return FromList(v)
	case skylark.Tuple:
		return FromTuple(v)
	case *skylark.Dict:
		return FromDict(v)
	case *skylark.Set:
		return FromSet(v)
	}
	return nil, fmt.Errorf("type %T is not a supported skylark type", v)
}

// MakeStringDict makes a StringDict from the given arg. The types supported are
// the same as ToValue.
func MakeStringDict(m map[string]interface{}) (skylark.StringDict, error) {
	dict := make(skylark.StringDict, len(m))
	for k, v := range m {
		val, err := ToValue(v)
		if err != nil {
			return nil, err
		}
		dict[k] = val
	}
	return dict, nil
}

// FromStringDict makes a map[string]interface{} from the given arg.  Any
// unconvertible values are ignored.
func FromStringDict(m skylark.StringDict) map[string]interface{} {
	ret := make(map[string]interface{}, len(m))
	for k, v := range m {
		val, err := FromValue(v)
		if err != nil {
			// we just ignore these, since they may be things like skylark
			// functions that we just can't represent.
			continue
		}
		ret[k] = val
	}
	return ret
}

// FromTuple converts a skylark.Tuple into a []interface{}.
func FromTuple(v skylark.Tuple) ([]interface{}, error) {
	vals := []skylark.Value(v)
	ret := make([]interface{}, len(vals))
	for i := range vals {
		val, err := FromValue(vals[i])
		if err != nil {
			return nil, err
		}
		ret[i] = val
	}
	return ret, nil
}

// MakeTuple makes a tuple from the given values.  The acceptable values are the
// same as ToValue.
func MakeTuple(v []interface{}) (skylark.Tuple, error) {
	vals := make([]skylark.Value, len(v))
	for i := range v {
		val, err := ToValue(v[i])
		if err != nil {
			return nil, err
		}
		vals[i] = val
	}
	return skylark.Tuple(vals), nil
}

// MakeList makes a list from the given values.  The acceptable values are the
// same as ToValue.
func MakeList(v []interface{}) (*skylark.List, error) {
	vals := make([]skylark.Value, len(v))
	for i := range v {
		val, err := ToValue(v[i])
		if err != nil {
			return nil, err
		}
		vals[i] = val
	}
	return skylark.NewList(vals), nil
}

// FromList creates a go slice from the given skylark list.
func FromList(l *skylark.List) ([]interface{}, error) {
	ret := make([]interface{}, 0, l.Len())
	var v skylark.Value
	i := l.Iterate()
	defer i.Done()
	for i.Next(&v) {
		val, err := FromValue(v)
		if err != nil {
			return nil, err
		}
		ret = append(ret, val)
	}
	return ret, nil
}

// MakeDict makes a Dict from the given map.  The acceptable keys and values are
// the same as ToValue.
func MakeDict(d map[interface{}]interface{}) (*skylark.Dict, error) {
	dict := skylark.Dict{}
	for k, v := range d {
		key, err := ToValue(k)
		if err != nil {
			return nil, err
		}
		val, err := ToValue(v)
		if err != nil {
			return nil, err
		}
		dict.Set(key, val)
	}
	return &dict, nil
}

// FromDict converts a skylark.Dict to a map[interface{}]interface{}
func FromDict(m *skylark.Dict) (map[interface{}]interface{}, error) {
	ret := make(map[interface{}]interface{}, m.Len())
	for _, k := range m.Keys() {
		key, err := FromValue(k)
		if err != nil {
			return nil, err
		}
		val, _, err := m.Get(k)
		if err != nil {
			return nil, err
		}
		ret[key] = val
	}
	return ret, nil
}

// MakeSet makes a Set from the given map.  The acceptable keys
// the same as ToValue.
func MakeSet(s map[interface{}]bool) (*skylark.Set, error) {
	set := skylark.Set{}
	for k := range s {
		key, err := ToValue(k)
		if err != nil {
			return nil, err
		}
		if err := set.Insert(key); err != nil {
			return nil, err
		}
	}
	return &set, nil
}

// FromSet converts a skylark.Set to a map[interface{}]bool
func FromSet(s *skylark.Set) (map[interface{}]bool, error) {
	ret := make(map[interface{}]bool, s.Len())
	var v skylark.Value
	i := s.Iterate()
	defer i.Done()
	for i.Next(&v) {
		val, err := FromValue(v)
		if err != nil {
			return nil, err
		}
		ret[val] = true
	}
	return ret, nil
}
