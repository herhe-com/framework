package elasticsearch

import (
	"bytes"
	"encoding/json"
	"errors"

	contractsearch "github.com/herhe-com/framework/contracts/search"
)

// SearchResult preserves Elasticsearch search metadata while decoding _source.
type SearchResult[T any] struct {
	Took         int64           `json:"took"`
	TimedOut     bool            `json:"timed_out"`
	Shards       json.RawMessage `json:"_shards"`
	Hits         SearchHits[T]   `json:"hits"`
	Aggregations json.RawMessage `json:"aggregations,omitempty"`
	PitID        string          `json:"pit_id,omitempty"`
}

// SearchHits contains total hit metadata and decoded hits.
type SearchHits[T any] struct {
	Total    TotalHits      `json:"total"`
	MaxScore *float64       `json:"max_score"`
	Hits     []SearchHit[T] `json:"hits"`
}

// TotalHits is the Elasticsearch total value and relation.
type TotalHits struct {
	Value    int64  `json:"value"`
	Relation string `json:"relation"`
}

// UnmarshalJSON supports both the Elasticsearch 6 numeric form and the newer object form.
func (total *TotalHits) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*total = TotalHits{}
		return nil
	}
	if data[0] != '{' {
		if err := json.Unmarshal(data, &total.Value); err != nil {
			return err
		}
		total.Relation = "eq"
		return nil
	}
	type totalHits TotalHits
	return json.Unmarshal(data, (*totalHits)(total))
}

// SearchHit preserves hit metadata and decodes its _source.
type SearchHit[T any] struct {
	Index     string                       `json:"_index"`
	ID        string                       `json:"_id"`
	Score     *float64                     `json:"_score"`
	Source    T                            `json:"_source"`
	Sort      []json.RawMessage            `json:"sort,omitempty"`
	Highlight map[string][]json.RawMessage `json:"highlight,omitempty"`
	Fields    json.RawMessage              `json:"fields,omitempty"`
}

// DecodeSearch decodes Elasticsearch hits without changing the raw response.
func DecodeSearch[T any](response *contractsearch.Response) (SearchResult[T], error) {
	var result SearchResult[T]
	if response == nil {
		return result, errors.New("elasticsearch: response is nil")
	}
	if err := json.Unmarshal(response.Body, &result); err != nil {
		return result, err
	}
	return result, nil
}
