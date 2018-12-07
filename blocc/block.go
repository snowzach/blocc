package blocc

import (
	"encoding/base64"
)

const (
	TypeBlock  = "block"
	TypeTx     = "tx"
	TypeMetric = "metric"
)

type Raw []byte

func (r Raw) MarshalJSON() ([]byte, error) {
	return []byte(`"` + base64.StdEncoding.EncodeToString(r) + `"`), nil
}

func (r Raw) UnmarshalJSON(in []byte) error {
	// Remove the beginning and ending "
	r = make([]byte, base64.StdEncoding.DecodedLen(len(in)))
	_, err := base64.StdEncoding.Decode(r, in)
	return err
}
