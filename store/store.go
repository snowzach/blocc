package store

import (
	"errors"
)

const (
	TypeBlock  = "block"
	TypeTx     = "tx"
	TypeMetric = "metric"
)

// ErrNotFound is a standard no found error
var ErrNotFound = errors.New("Not Found")
