package index

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
)

// float32Bytes is the width of one embedding component on the wire.
const float32Bytes = 4

// Vector is an embedding, stored on disk as a base64 blob of little-endian
// float32 values rather than as a JSON array of numbers.
//
// The encoding matters at the scale a backfilled vault reaches. Measured over
// ten thousand entries at 1536 dimensions, the index is 185 MB written as JSON
// number arrays and 80 MB written as blobs. Decoding is the larger win, because
// the whole index is read on every recall and chat: 2.8 seconds of float
// parsing becomes 0.75 seconds of copying.
type Vector []float32

// MarshalJSON renders the vector as a base64 blob. A nil vector marshals as
// JSON null so an entry still awaiting its embedding round trips unchanged.
func (v Vector) MarshalJSON() ([]byte, error) {
	if v == nil {
		return []byte("null"), nil
	}
	raw := make([]byte, len(v)*float32Bytes)
	for i, f := range v {
		binary.LittleEndian.PutUint32(raw[i*float32Bytes:], math.Float32bits(f))
	}
	return json.Marshal(base64.StdEncoding.EncodeToString(raw))
}

// UnmarshalJSON reads a vector written by MarshalJSON.
func (v *Vector) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*v = nil
		return nil
	}
	var encoded string
	if err := json.Unmarshal(data, &encoded); err != nil {
		return fmt.Errorf("decode embedding: expected a base64 blob: run `midden reindex` to rebuild the index: %w", err)
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return fmt.Errorf("decode embedding: %w", err)
	}
	if len(raw)%float32Bytes != 0 {
		return fmt.Errorf("decode embedding: %d bytes is not a whole number of float32 values", len(raw))
	}
	out := make(Vector, len(raw)/float32Bytes)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(raw[i*float32Bytes:]))
	}
	*v = out
	return nil
}
