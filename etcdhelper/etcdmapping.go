package etcdhelper

import (
	"reflect"
)

type etcdMapping struct {
	Index []int
}

func (em etcdMapping) ResolveValue(outer reflect.Value) reflect.Value {
	return outer.FieldByIndex(em.Index)
}

// Maps a given reflect type to a map of etcd mappings to easily get the
// destination struct value when processing the list of etcd KV responses.
func createEtcdMapping(typ reflect.Type) map[string]etcdMapping {
	if typ.Kind() != reflect.Struct {
		panic("currently only structs can be mapped")
	}

	mapping := make(map[string]etcdMapping)

	for _, field := range reflect.VisibleFields(typ) {
		name := field.Name

		entry := etcdMapping{
			Index: field.Index,
		}
		if tag, ok := field.Tag.Lookup("etcd"); ok {
			if tag == "-" { // skip field
				continue
			}
			name = tag
		}

		mapping[name] = entry
	}

	return mapping
}
