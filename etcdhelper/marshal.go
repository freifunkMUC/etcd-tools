package etcdhelper

import (
	"context"
	"reflect"
	"strconv"
	"strings"

	"go.etcd.io/etcd/client/v3"
)

func UnmarshalGet(ctx context.Context, kv clientv3.KV, prefix string, dest any) (uint, error) {
	// we do this here, as we need to control the sorting order for unmarshal
	// this allows us to sort the keys lexicographical, allowing us to use efficient recursion to fill sub structures
	resp, err := kv.Get(ctx, prefix, clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend))
	if err != nil {
		return 0, err
	}

	return unmarshalSortedGet(resp, prefix, reflect.ValueOf(dest))
}

func unmarshalSortedGet(resp *clientv3.GetResponse, prefix string, dest reflect.Value) (uint, error) {
	if dest.Kind() != reflect.Pointer {
		panic("unmarshalKVResponse expects a pointer type as the destination value")
	}
	val := dest.Elem()

	var appliedValues uint
	if val.Kind() == reflect.Map {
		if val.Type().Key().Kind() != reflect.String {
			panic("only maps with string keys are currently supported")
		}
		mapValTyp := val.Type().Elem()

		for len(resp.Kvs) > 0 {
			kv := resp.Kvs[0]
			if !strings.HasPrefix(string(kv.Key), prefix) {
				break
			}
			keyname := strings.TrimPrefix(string(kv.Key), prefix)

			splitted := strings.SplitN(keyname, "/", 2)
			if len(splitted) < 2 {
				// we currently don't support mapping map values that aren't structs, ignore
				resp.Kvs[0] = nil
				resp.Kvs = resp.Kvs[1:]
				continue
			}

			var value reflect.Value
			if mapValTyp.Kind() == reflect.Pointer {
				value = reflect.New(mapValTyp.Elem())
			} else {
				value = reflect.New(mapValTyp)
			}
			av, err := unmarshalSortedGet(resp, prefix+splitted[0]+"/", value)
			appliedValues += av
			if err != nil {
				return appliedValues, err
			}
			if mapValTyp.Kind() != reflect.Pointer {
				value = value.Elem()
			}
			val.SetMapIndex(reflect.ValueOf(splitted[0]), value)
		}
		return appliedValues, nil
	}

	mapping := createEtcdMapping(val.Type())

	for len(resp.Kvs) > 0 {
		kv := resp.Kvs[0]
		if !strings.HasPrefix(string(kv.Key), prefix) {
			break
		}

		keyname := strings.TrimPrefix(string(kv.Key), prefix)
		if _, ok := mapping[keyname]; !ok {
			// key not mapped to the go struct
			continue
		}
		field := mapping[keyname].ResolveValue(val)

		if field.Kind() == reflect.Pointer {
			// initialize ptr and switch field to the actual value
			ptr := reflect.New(field.Type().Elem())
			field.Set(ptr)
			field = field.Elem()
		}

		switch field.Kind() {
		case reflect.String:
			field.SetString(string(kv.Value))
			appliedValues++
		case reflect.Slice:
			if field.Type().Elem().Kind() != reflect.Uint8 {
				panic("currently only slices of byte/uint8 can be handled")
			}
			field.SetBytes(kv.Value)
		case reflect.Uint64:
			val, err := strconv.ParseUint(string(kv.Value), 10, 64)
			if err != nil {
				return appliedValues, err
			}
			field.SetUint(val)
		case reflect.Int64:
			val, err := strconv.ParseInt(string(kv.Value), 10, 64)
			if err != nil {
				return appliedValues, err
			}
			field.SetInt(val)
		default:
			panic("unmarshaling " + field.Kind().String() + " is not implemented")
		}
		resp.Kvs[0] = nil
		resp.Kvs = resp.Kvs[1:]
	}

	return appliedValues, nil

}

func Marshal(source any, prefix string) []clientv3.Op {
	val := reflect.ValueOf(source)
	for val.Kind() == reflect.Pointer {
		val = val.Elem()
	}

	mapping := createEtcdMapping(val.Type())
	ops := make([]clientv3.Op, 0, len(mapping))

entryloop:
	for key, entry := range mapping {
		field := entry.ResolveValue(val)

		for field.Kind() == reflect.Pointer {
			if field.IsNil() {
				continue entryloop
			}
			field = field.Elem()
		}

		switch field.Kind() {
		case reflect.String:
			ops = append(ops, clientv3.OpPut(prefix+key, field.String()))
		case reflect.Slice:
			if field.IsNil() {
				continue entryloop
			}
			if field.Type().Elem().Kind() != reflect.Uint8 {
				panic("currently only slices of byte/uint8 can be handled")
			}
			value := string(field.Interface().([]byte))
			ops = append(ops, clientv3.OpPut(prefix+key, value))
		case reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8, reflect.Uint:
			value := strconv.FormatUint(field.Uint(), 10)
			ops = append(ops, clientv3.OpPut(prefix+key, value))
		case reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8, reflect.Int:
			value := strconv.FormatInt(field.Int(), 10)
			ops = append(ops, clientv3.OpPut(prefix+key, value))
		default:
			panic("marshaling " + field.Kind().String() + " is not implemented")
		}
	}

	return ops
}
