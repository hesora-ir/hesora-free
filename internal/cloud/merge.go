package cloud

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
)

// MergeDocumentData merges independent JSON object fields against the last
// common snapshot. Same-field conflicts follow the caller's explicit policy.
func MergeDocumentData(baseJSON, localJSON, remoteJSON []byte, preferLocal bool) ([]byte, []string, error) {
	base, err := decodeMergeJSON(baseJSON)
	if err != nil {
		return nil, nil, fmt.Errorf("decode common cloud document: %w", err)
	}
	local, err := decodeMergeJSON(localJSON)
	if err != nil {
		return nil, nil, fmt.Errorf("decode local cloud document: %w", err)
	}
	remote, err := decodeMergeJSON(remoteJSON)
	if err != nil {
		return nil, nil, fmt.Errorf("decode remote cloud document: %w", err)
	}
	merged, _, conflicts := mergeJSONValue(base, true, local, true, remote, true, "", preferLocal)
	encoded, err := json.Marshal(merged)
	if err != nil {
		return nil, nil, err
	}
	sort.Strings(conflicts)
	return encoded, conflicts, nil
}

func decodeMergeJSON(raw []byte) (any, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return map[string]any{}, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, errors.New("multiple JSON values")
		}
		return nil, err
	}
	return value, nil
}

func mergeJSONValue(base any, baseOK bool, local any, localOK bool, remote any, remoteOK bool, path string, preferLocal bool) (any, bool, []string) {
	localChanged := !sameMergeValue(local, localOK, base, baseOK)
	// Missing fields are omitted by older clients when they do not understand
	// a newer optional property, so absence on the local side is not deletion.
	if !localOK && baseOK {
		localChanged = false
	}
	remoteChanged := !sameMergeValue(remote, remoteOK, base, baseOK)
	if !localChanged {
		return remote, remoteOK, nil
	}
	if !remoteChanged || sameMergeValue(local, localOK, remote, remoteOK) {
		return local, localOK, nil
	}
	baseMap, baseIsMap := base.(map[string]any)
	localMap, localIsMap := local.(map[string]any)
	remoteMap, remoteIsMap := remote.(map[string]any)
	if localIsMap && remoteIsMap {
		if !baseIsMap {
			baseMap = map[string]any{}
		}
		keys := map[string]struct{}{}
		for key := range baseMap {
			keys[key] = struct{}{}
		}
		for key := range localMap {
			keys[key] = struct{}{}
		}
		for key := range remoteMap {
			keys[key] = struct{}{}
		}
		ordered := make([]string, 0, len(keys))
		for key := range keys {
			ordered = append(ordered, key)
		}
		sort.Strings(ordered)
		out := make(map[string]any, len(ordered))
		conflicts := []string{}
		for _, key := range ordered {
			b, bok := baseMap[key]
			l, lok := localMap[key]
			r, rok := remoteMap[key]
			childPath := key
			if path != "" {
				childPath = path + "." + key
			}
			value, valueOK, childConflicts := mergeJSONValue(b, bok, l, lok, r, rok, childPath, preferLocal)
			if valueOK {
				out[key] = value
			}
			conflicts = append(conflicts, childConflicts...)
		}
		return out, true, conflicts
	}
	if path == "" {
		path = "$"
	}
	if preferLocal {
		return local, localOK, []string{path}
	}
	return remote, remoteOK, []string{path}
}

func sameMergeValue(a any, aOK bool, b any, bOK bool) bool {
	return aOK == bOK && reflect.DeepEqual(a, b)
}
