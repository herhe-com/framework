package elasticsearch

import (
	"testing"

	contractsearch "github.com/herhe-com/framework/contracts/search"
)

func TestDecodeSearchPreservesHitMetadata(t *testing.T) {
	type User struct {
		Name string `json:"name"`
	}
	result, err := DecodeSearch[User](&contractsearch.Response{Body: []byte(`{"took":2,"hits":{"total":{"value":1,"relation":"eq"},"max_score":1.5,"hits":[{"_index":"users","_id":"1","_score":1.5,"_source":{"name":"Alice"},"sort":[10],"highlight":{"name":["<em>Alice</em>"]}}]},"aggregations":{"roles":{"buckets":[]}}}`)})
	if err != nil {
		t.Fatal(err)
	}
	if result.Took != 2 || result.Hits.Total.Value != 1 || len(result.Hits.Hits) != 1 {
		t.Fatalf("result = %#v", result)
	}
	hit := result.Hits.Hits[0]
	if hit.ID != "1" || hit.Source.Name != "Alice" || len(hit.Sort) != 1 || len(hit.Highlight["name"]) != 1 || len(result.Aggregations) == 0 {
		t.Fatalf("hit = %#v", hit)
	}
}

func TestDecodeSearchSupportsLegacyNumericTotal(t *testing.T) {
	result, err := DecodeSearch[map[string]any](&contractsearch.Response{Body: []byte(`{"hits":{"total":3,"hits":[]}}`)})
	if err != nil {
		t.Fatal(err)
	}
	if result.Hits.Total.Value != 3 || result.Hits.Total.Relation != "eq" {
		t.Fatalf("total = %#v", result.Hits.Total)
	}
}
