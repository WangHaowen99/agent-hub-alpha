package main

// Adapted from PR #25, internal/e2e/t2host_test.go put/queryForReconciliation.
// Storage and transaction ownership remain with the PG caller. This package
// has no cache, file, goroutine, scheduler or mutable persistence of its own.

// Record separates the bound input from the provider receipt and result body.
type Record[T any] struct {
	InputDigest   string
	ContentDigest string
	Value         T
}

// QueryForReconciliation is a read face, never a permission to re-execute.
// An absent result does not imply that no remote request was accepted.
func QueryForReconciliation[T any](stored *Record[T], inputDigest string) (committed, serve bool, value T) {
	if stored == nil {
		return false, false, value
	}
	return true, stored.InputDigest == inputDigest, stored.Value
}

// CanReuseCommit forbids same-receipt replacement and keeps o independent of a.
// The existing record is returned unchanged only for identical input/content.
func CanReuseCommit[T any](stored *Record[T], inputDigest, contentDigest string) bool {
	committed, serve, _ := QueryForReconciliation(stored, inputDigest)
	return committed && serve && stored.ContentDigest == contentDigest
}
