package blocc

import (
	"bytes"
	"encoding/base64"
	"io"
)

const (
	TypeBlock  = "block"
	TypeTx     = "tx"
	TypeMetric = "metric"
)

type Raw struct {
	bytes.Buffer
}

func (r *Raw) MarshalJSON() ([]byte, error) {
	return []byte(`"` + base64.StdEncoding.EncodeToString(r.Bytes()) + `"`), nil
}

func (r *Raw) UnmarshalJSON(in []byte) error {
	// Remove the beginning and ending "
	in = bytes.Trim(in, `"`)
	r.Reset()
	_, err := io.Copy(r, base64.NewDecoder(base64.StdEncoding, bytes.NewBuffer(in)))
	if err != nil {
		return err
	}
	return nil
}
