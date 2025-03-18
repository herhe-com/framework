package queue

type BasicError struct {
	Exchange string `json:"exchange"`
	Queue    string `json:"queue"`
	Route    string `json:"route"`
	Message  string `json:"message"`
	Error    string `json:"error"`
}
