// Package flock wraps advisory file locks for safe concurrent appends.
// Unix builds use flock(2); Windows builds use LockFileEx.
package flock
