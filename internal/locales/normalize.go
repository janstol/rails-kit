package locales

import (
	"fmt"
	"strconv"
	"time"
)

// normalizeValue rewrites every map[interface{}]interface{} the YAML decoder
// produced into a map[string]interface{}, recursing through nested maps and
// sequences. yaml.v3 falls back to an interface-keyed map for any mapping with
// even one non-string key (decode.go isStringMap), and every consumer in this
// package -- deepMerge, Navigate, collectScopes, printNode -- only recognizes
// string-keyed maps.
func normalizeValue(v interface{}) interface{} {
	switch m := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(m))
		for k, val := range m {
			out[k] = normalizeValue(val)
		}
		return out
	case map[interface{}]interface{}:
		return normalizeInterfaceMap(m)
	case []interface{}:
		out := make([]interface{}, len(m))
		for i, item := range m {
			out[i] = normalizeValue(item)
		}
		return out
	default:
		return v
	}
}

// normalizeInterfaceMap coerces every key to a string via normalizeKey.
//
// Collision policy: after coercion, two differently-authored keys can
// collapse to one -- e.g. `True:` (resolves to the bool true) and `"true":`
// (a plain string) both render as "true". (Two spellings of the *same*
// resolved value, like `404:` and `"404":`, don't reach this code at all:
// yaml.v3's own duplicate-key check compares raw scalar text and rejects
// the file before decoding gets this far.) Go map iteration order is
// randomized, so the winner is chosen explicitly rather than left to
// whichever raw key is visited last:
//  1. a key authored as a YAML string beats any coerced (non-string) key
//     (this matches what Rails' I18n lookup would find, since it looks up
//     by symbol);
//  2. if two coerced keys still collide, the one whose raw key renders as
//     the greater "%T:%v" string wins.
//
// Both rules compare only the two colliding raw keys, so the result does
// not depend on map iteration order. YAML itself treats these as distinct
// keys, so this only arises in pathological files.
func normalizeInterfaceMap(m map[interface{}]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	owners := make(map[string]interface{}, len(m)) // normalized key -> raw key currently backing out[key]
	for k, val := range m {
		key := normalizeKey(k)
		if owner, ok := owners[key]; ok && !rawKeyWins(k, owner) {
			continue
		}
		out[key] = normalizeValue(val)
		owners[key] = k
	}
	return out
}

// rawKeyWins reports whether candidate should win over current when both
// normalize to the same string key. See normalizeInterfaceMap for the policy.
func rawKeyWins(candidate, current interface{}) bool {
	_, candidateIsStr := candidate.(string)
	_, currentIsStr := current.(string)
	if candidateIsStr != currentIsStr {
		return candidateIsStr
	}
	// Two distinct keys of the same underlying (comparable) type and value
	// can't both exist in a Go map, so candidateIsStr && currentIsStr here
	// is unreachable. Break the tie between two coerced keys deterministically.
	return fmt.Sprintf("%T:%v", candidate, candidate) > fmt.Sprintf("%T:%v", current, current)
}

// normalizeKey renders a decoded YAML scalar key as its string form. It
// covers exactly the scalar types yaml.v3 yields for a plain scalar
// (resolve.go:181-197).
func normalizeKey(k interface{}) string {
	switch v := k.(type) {
	case string:
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'g', -1, 64)
	case nil:
		return "null"
	case time.Time:
		if v.Location() == time.UTC && v.Equal(v.Truncate(24*time.Hour)) {
			return v.Format("2006-01-02")
		}
		return v.Format(time.RFC3339)
	default:
		return fmt.Sprintf("%v", v)
	}
}
