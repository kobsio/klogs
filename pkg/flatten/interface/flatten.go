package flatten

import (
	"fmt"
	"strconv"
)

// Flatten generates a flat map from a nested one. The nested map may include values of type map, slice and scalar, but
// not struct. Keys in the flat map will be a compound of descending map keys and slice iterations.
func Flatten(nested map[interface{}]interface{}) (map[string]interface{}, error) {
	flatmap := make(map[string]interface{})

	err := flatten(true, flatmap, nested, "")
	if err != nil {
		return nil, err
	}

	return flatmap, nil
}

func flatten(top bool, flatMap map[string]interface{}, nested interface{}, prefix string) error {
	assign := func(newKey string, v interface{}) error {
		switch v.(type) {
		case map[interface{}]interface{}, []interface{}:
			if err := flatten(false, flatMap, v, newKey); err != nil {
				return err
			}
		default:
			flatMap[newKey] = v
		}

		return nil
	}

	switch nested.(type) {
	case map[interface{}]interface{}:
		for k, v := range nested.(map[interface{}]interface{}) {
			newKey := enkey(top, prefix, k.(string))
			assign(newKey, v)
		}
	case []interface{}:
		for i, v := range nested.([]interface{}) {
			newKey := enkey(top, prefix, strconv.Itoa(i))
			assign(newKey, v)
		}
	default:
		// Nested input must be a slice or map, for everything else we are returning an error.
		return fmt.Errorf("invalid input: must be a map or slice")
	}

	return nil
}

func enkey(top bool, prefix, subkey string) string {
	key := prefix

	if top {
		key += subkey
	} else {
		key += "_" + subkey
	}

	return key
}
