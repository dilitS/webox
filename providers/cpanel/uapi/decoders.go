package uapi

import (
	"encoding/json"
	"sort"
)

// decodeListResponse is the shared shape-tolerant decoder used by
// modules that return a list of rows. cPanel UAPI exposes the
// payload as one of three shapes depending on version:
//
//  1. Object wrapper: {"<key>": [...]}.
//  2. Top-level array: [...].
//  3. Map keyed by row name (legacy WHM): {"row-a": {...}, ...}.
//
// `wrapperKey` is the JSON key for shape #1 (e.g. "applications",
// "databases", "keys"). `nameFromKey` is invoked for shape #3 so
// the legacy map key can be promoted to the row's Name field —
// SSL.list_keys never used the legacy map shape, so its caller
// passes nil and shape #3 is treated as an unrecognised payload.
//
// The result is sorted by `sortLess` so tests, TUI rows, and
// snapshots stay stable across cPanel versions. Returning the
// generic []T concretely (via the type parameter) lets the caller
// avoid an extra cast at the call site.
func decodeListResponse[T any](
	raw []byte,
	wrapperKey string,
	nameFromKey func(name string, row *T),
	sortLess func(a, b T) bool,
	notFound error,
) ([]T, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	wrapper := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &wrapper); err == nil {
		if inner, ok := wrapper[wrapperKey]; ok {
			rows := []T{}
			if err := json.Unmarshal(inner, &rows); err == nil && len(rows) > 0 {
				sortStable(rows, sortLess)
				return rows, nil
			}
		}
	}
	rows := []T{}
	if err := json.Unmarshal(raw, &rows); err == nil && len(rows) > 0 {
		sortStable(rows, sortLess)
		return rows, nil
	}
	if nameFromKey == nil {
		if notFound != nil {
			return nil, notFound
		}
		return nil, nil
	}
	asMap := map[string]T{}
	if err := json.Unmarshal(raw, &asMap); err != nil {
		if notFound != nil {
			return nil, notFound
		}
		return nil, err
	}
	rows = rows[:0]
	for name, row := range asMap {
		row := row
		nameFromKey(name, &row)
		rows = append(rows, row)
	}
	sortStable(rows, sortLess)
	return rows, nil
}

// sortStable is a small generic wrapper around sort.SliceStable so
// the decoders read top-to-bottom without an inline closure
// allocation at every call site.
func sortStable[T any](rows []T, less func(a, b T) bool) {
	sort.SliceStable(rows, func(i, j int) bool { return less(rows[i], rows[j]) })
}

// decodePassengerApps decodes PassengerApps.list_applications via
// the shared shape-tolerant decoder. The legacy WHM shape keys
// each app by name (and the inner object omits Name), so we
// promote the key when present.
func decodePassengerApps(raw []byte) ([]PassengerApp, error) {
	return decodeListResponse(
		raw,
		"applications",
		func(name string, row *PassengerApp) {
			if row.Name == "" {
				row.Name = name
			}
		},
		func(a, b PassengerApp) bool { return a.Name < b.Name },
		nil,
	)
}

// decodeMysqlDatabases decodes Mysql.list_databases via the
// shared shape-tolerant decoder. The legacy WHM shape keys each
// database by name; the modern shape uses the `databases` array.
func decodeMysqlDatabases(raw []byte) ([]MysqlDatabase, error) {
	return decodeListResponse(
		raw,
		"databases",
		func(name string, row *MysqlDatabase) {
			if row.Name == "" {
				row.Name = name
			}
		},
		func(a, b MysqlDatabase) bool { return a.Name < b.Name },
		nil,
	)
}

// decodeSSLKeys decodes SSL.list_keys via the shared decoder.
// Unlike the previous two modules, SSL.list_keys never used a
// legacy map-keyed shape, so we surface unrecognised payloads as
// [ErrUnknownSSLShape] rather than silently returning empty.
func decodeSSLKeys(raw []byte) ([]SSLKey, error) {
	return decodeListResponse(
		raw,
		"keys",
		nil,
		func(a, b SSLKey) bool { return a.FriendlyName < b.FriendlyName },
		ErrUnknownSSLShape,
	)
}
