package store

import (
	"bytes"
	"encoding/base64"
)

type Block struct {
	Type        string             `json:"type"`
	Symbol      string             `json:"symbol"`
	BlockId     string             `json:"blockId"`
	Height      int64              `json:"height"`
	PrevBlockId string             `json:"prevBlockId"`
	Time        int64              `json:"time"`
	Addresses   []string           `json:"address"`
	TxIds       []string           `json:"txId"`
	Raw         *Raw               `json:"raw"`
	Tags        map[string]string  `json:"tag"`
	Metrics     map[string]float64 `json:"metric"`
}

type Tx struct {
	Type      string             `json:"type"`
	Symbol    string             `json:"symbol"`
	BlockId   string             `json:"blockId"`
	TxId      string             `json:"txId"`
	Height    int64              `json:"height"`
	Time      int64              `json:"time"`
	BlockTime int64              `json:"blockTime"`
	Addresses []string           `json:"address"`
	Raw       *Raw               `json:"raw"`
	Tags      map[string]string  `json:"tag"`
	Metrics   map[string]float64 `json:"metric"`
}

type Raw struct {
	bytes.Buffer
}

func (r *Raw) MarshalJSON() ([]byte, error) {
	return []byte(`"` + base64.StdEncoding.EncodeToString(r.Bytes()) + `"`), nil
}
