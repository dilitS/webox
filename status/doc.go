// Package status implements the three-tier Stale-While-Revalidate
// cache that powers the Webox dashboard.
//
// The cache returns stale data immediately while triggering a
// background refresh, dedupes parallel fetches with
// golang.org/x/sync/singleflight, and respects per-key TTLs from
// docs/DESIGN.md §8. The cold-cache dashboard math is documented in
// docs/adr/0005-cache-statusow-projektow.md.
package status
