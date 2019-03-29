package store

import (
	"time"
)

const (
	// Used by things that have offset/count to indicate we want as much as we can get
	CountMax = -1
)

type DistCache interface {
	Set(bucket string, key string, value interface{}, expires time.Duration) error
	GetScan(bucket string, key string, dest interface{}) error
	GetBytes(bucket string, key string) ([]byte, error)
	Del(bucket string, key string) error
	Clear(bucket string) error
}
