package cache

import "github.com/sharp-tools/sharp-tools/internal/db"

// CacheResult indicates whether a cache lookup found a match
type CacheResult int

const (
	CacheMiss CacheResult = iota
	CacheHit
)

// LookupResult represents the result of a cache lookup
type LookupResult struct {
	Result CacheResult
	Tool   *db.Tool // nil on miss
}