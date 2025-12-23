package meilisearch

import (
	"github.com/herhe-com/framework/contracts/search"
	"github.com/meilisearch/meilisearch-go"
)

func (receiver Client) Index(index, data string) (err error) {

	_, err = receiver.client.CreateIndex(&meilisearch.IndexConfig{
		Uid:        receiver.index(index),
		PrimaryKey: data,
	})

	return err
}

func (receiver Client) Del(index string) (err error) {

	idx := receiver.client.Index(receiver.index(index))

	_, err = idx.DeleteAllDocuments(nil)

	return err
}

func (receiver Client) Save(index, key string, doc map[string]any) (err error) {

	idx := receiver.client.Index(receiver.index(index))

	_, err = idx.AddDocuments([]any{doc}, &meilisearch.DocumentOptions{
		PrimaryKey: &key,
	})

	return err
}

func (receiver Client) Document(index, id string) (document map[string]any, err error) {

	idx := receiver.client.Index(receiver.index(index))

	if err = idx.GetDocument(id, nil, &document); err != nil {
		return nil, err
	}

	return nil, nil
}

func (receiver Client) Delete(index, id string) error {

	idx := receiver.client.Index(receiver.index(index))

	_, err := idx.DeleteDocument(id, nil)

	return err
}

func (receiver Client) Search(index, query string, request search.Request) (result *search.Paginate, err error) {

	idx := receiver.client.Index(receiver.index(index))

	resp, err := idx.Search(query, &meilisearch.SearchRequest{
		Filter: request.Condition,
		Limit:  int64(request.Limit),
		Offset: int64(request.Offset),
	})

	if err != nil {
		return nil, err
	}

	result = &search.Paginate{
		Page:  (request.Limit + request.Offset) / request.Limit,
		Size:  request.Limit,
		Total: resp.TotalHits,
		Data:  make([]map[string]any, 0),
	}

	for _, item := range resp.Hits {

		var data map[string]any

		if err = item.DecodeInto(&data); err != nil {
			result.Data = append(result.Data, data)
		}
	}

	return result, nil
}

func (receiver Client) index(index string) string {

	if receiver.prefix != "" {
		index = receiver.prefix + index
	}

	return index
}

func (receiver Client) Dri() string {
	return search.DriverMeiliSearch
}

func (receiver Client) Ping() (bool, error) {

	ok := receiver.client.IsHealthy()

	return ok, nil
}
