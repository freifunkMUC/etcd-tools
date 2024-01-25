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
