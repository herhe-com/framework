package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/herhe-com/framework/contracts/search"
	"strings"
)

func (receiver Client) Index(index, data string) error {

	resp, err := receiver.client.Indices.Create(
		receiver.index(index),
		receiver.client.Indices.Create.WithContext(context.Background()),
		receiver.client.Indices.Create.WithBody(strings.NewReader(data)),
	)

	if err != nil {
		return err
	}

	if _, err = receiver.Response(resp); err != nil {
		return err
	}

	return nil
}

func (receiver Client) Del(index string) error {

	resp, err := receiver.client.Indices.Delete([]string{receiver.index(index)})

	if err != nil {
		return err
	}

	if _, err = receiver.Response(resp); err != nil {
		return err
	}

	return nil
}

func (receiver Client) Save(index, key string, doc map[string]any) error {

	id, ok := doc[key]

	if !ok {
		return errors.New("key not found")
	}

	body := &bytes.Buffer{}

	if err := json.NewEncoder(body).Encode(doc); err != nil {
		return err
	}

	response, err := receiver.client.Index(receiver.index(index), body, receiver.client.Index.WithDocumentID(fmt.Sprintf("%v", id)))

	if err != nil {
		return err
	}

	var resp string

	if resp, err = receiver.Response(response); err != nil {
		return err
	}

	var res HandleResponse

	if err = json.Unmarshal([]byte(resp), &res); err != nil {
		return err
	}

	if res.Shards.Successful == 0 {
		return errors.New("save failed")
	}

	return nil
}

func (receiver Client) Document(index, id string) (document map[string]any, err error) {

	response, err := receiver.client.Get(receiver.index(index), id)

	if err != nil {
		return nil, err
	}

	var resp string

	if resp, err = receiver.Response(response); err != nil {
		return nil, err
	}

	var res DocumentResponse

	if err = json.Unmarshal([]byte(resp), &res); err != nil {
		return nil, err
	}

	if !res.Found {
		return nil, errors.New("document not found")
	}

	return res.Source, nil
}

func (receiver Client) Delete(index, id string) error {

	response, err := receiver.client.Delete(receiver.index(index), id)

	if err != nil {
		return err
	}

	var resp string

	if resp, err = receiver.Response(response); err != nil {
		return err
	}

	var res HandleResponse

	if err = json.Unmarshal([]byte(resp), &res); err != nil {
		return err
	}

	if res.Shards.Successful == 0 {
		return errors.New("delete failed")
	}

	return nil
}

func (receiver Client) Search(index, query string, request search.Request) (result *search.Paginate, err error) {

	data := map[string]any{
		"query": map[string]any{
			"match": map[string]any{
				"name": query,
			},
		},
		"from": request.Offset,
		"size": request.Limit,
	}

	if request.Condition != "" {

		var condition map[string]any

		if err = json.Unmarshal([]byte(request.Condition), &condition); err != nil {
			return nil, err
		}

		data["query"] = condition
	}

	document, _ := json.Marshal(data)

	response, err := receiver.client.Search(
		receiver.client.Search.WithIndex(receiver.index(index)),
		receiver.client.Search.WithBody(bytes.NewReader(document)),
	)

	if err != nil {
		return nil, err
	}

	var resp string

	if resp, err = receiver.Response(response); err != nil {
		return nil, err
	}

	var res SearchResponse

	if err = json.Unmarshal([]byte(resp), &res); err != nil {
		return nil, err
	}

	result = &search.Paginate{
		Page:  (request.Limit + request.Offset) / request.Limit,
		Size:  request.Limit,
		Total: res.Hits.Total.Value,
		Data:  make([]map[string]any, 0),
	}

	for _, item := range res.Hits.Hits {
		result.Data = append(result.Data, item.Source)
	}

	return result, nil
}

func (receiver Client) index(index string) string {

	if receiver.prefix != "" {
		index = receiver.prefix + index
	}

	return index
}

func (receiver Client) Response(response *esapi.Response) (resp string, err error) {

	if response == nil {
		return resp, errors.New("response is nil")
	}

	buf := new(bytes.Buffer)

	if _, err = buf.ReadFrom(response.Body); err != nil {
		return resp, err
	}

	var res ErrorResponse

	_ = json.Unmarshal(buf.Bytes(), &response)

	if res.Status > 0 {
		return "", errors.New(res.Error.Reason)
	}

	return buf.String(), nil
}

func (receiver Client) Dri() string {
	return search.DriverElasticSearch
}

func (receiver Client) Ping() (bool, error) {

	response, err := receiver.client.Ping()

	if err != nil {
		return false, err
	}

	return !response.IsError(), nil
}
