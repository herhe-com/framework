package elasticsearch

type ErrorResponse struct {
	Error struct {
		Header struct {
			WWWAuthenticate string `json:"WWW-Authenticate"`
		} `json:"header"`
		Reason    string `json:"reason"`
		RootCause []struct {
			Header struct {
				WWWAuthenticate string `json:"WWW-Authenticate"`
			} `json:"header"`
			Reason string `json:"reason"`
			Type   string `json:"type"`
		} `json:"root_cause"`
		Type string `json:"type"`
	} `json:"error"`
	Status int `json:"status"`
}

type HandleResponse struct {
	Index   string `json:"_index"`
	Type    string `json:"_type"`
	Id      string `json:"_id"`
	Version int    `json:"_version"`
	Result  string `json:"result"`
	Shards  struct {
		Total      int `json:"total"`
		Successful int `json:"successful"`
		Failed     int `json:"failed"`
	} `json:"_shards"`
	SeqNo       int `json:"_seq_no"`
	PrimaryTerm int `json:"_primary_term"`
}

type SearchResponse struct {
	Shards struct {
		Failed     int `json:"failed"`
		Skipped    int `json:"skipped"`
		Successful int `json:"successful"`
		Total      int `json:"total"`
	} `json:"_shards"`
	Hits struct {
		Hits []struct {
			Id     string         `json:"_id"`
			Index  string         `json:"_index"`
			Score  float64        `json:"_score"`
			Source map[string]any `json:"_source"`
			Type   string         `json:"_type"`
		} `json:"hits"`
		MaxScore float64 `json:"max_score"`
		Total    struct {
			Relation string `json:"relation"`
			Value    int64  `json:"value"`
		} `json:"total"`
	} `json:"hits"`
	TimedOut bool `json:"timed_out"`
	Took     int  `json:"took"`
}

type DocumentResponse struct {
	Index       string         `json:"_index"`
	Type        string         `json:"_type"`
	Id          string         `json:"_id"`
	Version     int            `json:"_version"`
	SeqNo       int            `json:"_seq_no"`
	PrimaryTerm int            `json:"_primary_term"`
	Found       bool           `json:"found"`
	Source      map[string]any `json:"_source"`
}
