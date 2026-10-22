package workbench

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"motty/internal/configstore"
)

// savedFieldChange names one top-level field of savedState that differs
// between two snapshots, and carries its current value for the incremental
// writer.
type savedFieldChange struct {
	name  string
	value any
}

// changedSavedFields lists the top-level fields that differ between two
// snapshots. Comparing field by field tells a one-field save from a mixed one
// and lets the incremental writer encode only what changed.
func changedSavedFields(previous, current savedState) []savedFieldChange {
	before, after := reflect.ValueOf(previous), reflect.ValueOf(current)
	kind := before.Type()
	var changes []savedFieldChange
	for index := 0; index < kind.NumField(); index++ {
		if reflect.DeepEqual(before.Field(index).Interface(), after.Field(index).Interface()) {
			continue
		}
		name, _, _ := strings.Cut(kind.Field(index).Tag.Get("json"), ",")
		changes = append(changes, savedFieldChange{name: name, value: after.Field(index).Interface()})
	}
	return changes
}

// writeChangedFields stores several changed top-level fields in one commit.
// Every record and device value that did not change is reused from base; only
// the changed lists, settings and device keys are encoded. The host group list
// is split the same way as the full encoder: records carry no collapsed flag,
// and the collapsed names go into this device's own JSON. handled is false only
// for a state version field.
func writeChangedFields(store *configstore.Store, base *configBase, defaults configDefaults, changes []savedFieldChange) (shared, handled bool, err error) {
	entries := make([]configstore.Entry, 0, len(base.payloads)+len(changes))
	for key, payload := range base.payloads {
		entries = append(entries, configstore.Entry{Kind: key.Kind, ID: key.ID, Payload: payload})
	}
	local := map[string]json.RawMessage{}
	if len(base.local) > 0 {
		if err := json.Unmarshal(base.local, &local); err != nil {
			return false, true, err
		}
	}
	localChanged := false
	nextOrd := base.maxOrd
	for _, change := range changes {
		if change.name == "host_groups" {
			entity, _ := entityByField(change.name)
			groups := change.value.([]savedHostGroup)
			collapsed := make([]string, 0, len(groups))
			stripped := make([]savedHostGroup, len(groups))
			for index, group := range groups {
				if group.Collapsed {
					collapsed = append(collapsed, group.Name)
				}
				group.Collapsed = false // a device's own view, not the shared list
				stripped[index] = group
			}
			entries = withoutKind(entries, entity.kind)
			entries, nextOrd, err = appendEntityEntries(entries, base, defaults, entity, stripped, nextOrd)
			if err != nil {
				return false, true, err
			}
			changed, err := setLocalField(local, collapsedGroupsKey, collapsed)
			if err != nil {
				return false, true, err
			}
			localChanged = localChanged || changed
			continue
		}
		if change.name == "ai_providers" {
			// A nil provider list means "not read yet", which splitState keeps as
			// the records already in base.
			value := reflect.ValueOf(change.value)
			if value.Kind() == reflect.Ptr && value.IsNil() {
				continue
			}
		}
		entity, isEntity := entityByField(change.name)
		switch {
		case isEntity:
			entries = withoutKind(entries, entity.kind)
			entries, nextOrd, err = appendEntityEntries(entries, base, defaults, entity, change.value, nextOrd)
			if err != nil {
				return false, true, err
			}
		case syncedSettings[change.name]:
			entries = withoutSetting(entries, change.name)
			entries, err = appendSettingEntry(entries, base, defaults, change.name, change.value)
			if err != nil {
				return false, true, err
			}
		case stateVersionFields[change.name]:
			return false, false, nil
		default:
			changed, err := setLocalField(local, change.name, change.value)
			if err != nil {
				return false, true, err
			}
			localChanged = localChanged || changed
		}
	}
	localBytes := base.local
	if localChanged {
		localBytes, err = json.Marshal(local)
		if err != nil {
			return false, true, err
		}
	}
	sortEntries(entries)
	shared, err = writeEntries(store, base, entries, localBytes)
	if err == nil {
		base.maxOrd = nextOrd
	}
	return shared, true, err
}

func entityByField(name string) (entityField, bool) {
	for _, entity := range entityFields {
		if entity.field == name {
			return entity, true
		}
	}
	return entityField{}, false
}

// appendEntityEntries appends the current records of one changed list to
// entries. It uses the same id, ordering, default and delete rules as
// splitState.
func appendEntityEntries(entries []configstore.Entry, base *configBase, defaults configDefaults, entity entityField, value any, nextOrd int64) ([]configstore.Entry, int64, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, nextOrd, err
	}
	var items []json.RawMessage
	if entity.single {
		if !bytes.Equal(raw, []byte("null")) {
			items = []json.RawMessage{raw}
		}
	} else if err := json.Unmarshal(raw, &items); err != nil {
		return nil, nextOrd, err
	}
	used := map[string]bool{}
	for _, item := range items {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(item, &fields); err != nil {
			return nil, nextOrd, err
		}
		id := boundKey(entity.key(fields))
		if id == "" {
			continue // nothing names it, so nothing can be synced about it
		}
		unique := id
		for number := 2; used[unique]; number++ {
			unique = fmt.Sprintf("%s#%d", id, number)
		}
		id = unique
		used[id] = true
		key := configstore.Key{Kind: entity.kind, ID: id}
		if _, held := base.payloads[key]; !held && defaults[key] == string(item) {
			continue
		}
		ord, known := base.ords[key]
		if !known {
			nextOrd++
			ord = nextOrd
		}
		payload, err := json.Marshal(recordPayload{Ord: ord, Data: item})
		if err != nil {
			return nil, nextOrd, err
		}
		entries = append(entries, configstore.Entry{Kind: entity.kind, ID: id, Payload: payload})
	}
	return entries, nextOrd, nil
}

// appendSettingEntry appends one changed shared setting. A setting left at its
// zero value has no record, exactly as the omitted field in splitState's JSON
// has none, so no entry is added and the base's old record is deleted.
func appendSettingEntry(entries []configstore.Entry, base *configBase, defaults configDefaults, name string, value any) ([]configstore.Entry, error) {
	omitted := reflect.ValueOf(value).IsZero()
	if v := reflect.ValueOf(value); v.Kind() == reflect.Slice || v.Kind() == reflect.Map {
		omitted = v.Len() == 0
	}
	if omitted {
		return entries, nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	key := configstore.Key{Kind: kindSetting, ID: name}
	if _, held := base.payloads[key]; !held && defaults[key] == string(data) {
		return entries, nil
	}
	payload, err := json.Marshal(recordPayload{Data: data})
	if err != nil {
		return nil, err
	}
	return append(entries, configstore.Entry{Kind: key.Kind, ID: key.ID, Payload: payload}), nil
}

// setLocalField updates or drops one key of this device's own JSON object. It
// reports whether the object changed.
func setLocalField(local map[string]json.RawMessage, name string, value any) (bool, error) {
	omitted := reflect.ValueOf(value).IsZero()
	if v := reflect.ValueOf(value); v.Kind() == reflect.Slice || v.Kind() == reflect.Map {
		omitted = v.Len() == 0
	}
	if omitted {
		if _, present := local[name]; !present {
			return false, nil
		}
		delete(local, name)
		return true, nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return false, err
	}
	if bytes.Equal(local[name], data) {
		return false, nil
	}
	local[name] = data
	return true, nil
}

func withoutKind(entries []configstore.Entry, kind string) []configstore.Entry {
	kept := entries[:0]
	for _, entry := range entries {
		if entry.Kind != kind {
			kept = append(kept, entry)
		}
	}
	return kept
}

func withoutSetting(entries []configstore.Entry, name string) []configstore.Entry {
	kept := entries[:0]
	for _, entry := range entries {
		if entry.Kind != kindSetting || entry.ID != name {
			kept = append(kept, entry)
		}
	}
	return kept
}

func sortEntries(entries []configstore.Entry) {
	sort.Slice(entries, func(left, right int) bool {
		if entries[left].Kind != entries[right].Kind {
			return entries[left].Kind < entries[right].Kind
		}
		return entries[left].ID < entries[right].ID
	})
}
