package etcdhelper

import (
	"reflect"
	"strconv"
	"strings"

	"go.etcd.io/etcd/client/v3"
)

func UnmarshalKVResponse(resp *clientv3.GetResponse, dest any, prefix string) error {
	if reflect.TypeOf(dest).Kind() != reflect.Pointer {
		panic("unmarshalKVResponse expects a pointer type as the destination value")
	}
	val := reflect.ValueOf(dest).Elem()

	mapping := createEtcdMapping(val.Type())

	for _, kv := range resp.Kvs {
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
		case reflect.Slice:
			if field.Type().Elem().Kind() != reflect.Uint8 {
				panic("currently only slices of byte/uint8 can be handled")
			}
			field.SetBytes(kv.Value)
		case reflect.Uint64:
			val, err := strconv.ParseUint(string(kv.Value), 10, 64)
			if err != nil {
				return err
			}
			field.SetUint(val)
		case reflect.Int64:
			val, err := strconv.ParseInt(string(kv.Value), 10, 64)
			if err != nil {
				return err
			}
			field.SetInt(val)
		default:
			panic("unmarshaling " + field.Kind().String() + " is not implemented")
		}
	}

	return nil
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
