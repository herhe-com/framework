package response

// Response is the standard JSON envelope returned by every HTTP endpoint.
//
// The HTTP status code is always 200 for JSON endpoints; the business result is
// carried by Code. Data is only filled for successful responses of endpoints
// that return a payload.
type Response[T any] struct {
	// Code is the business status code: 20000 means success, every other value is
	// a failure code (40000 invalid request, 40100 unauthenticated, 40300
	// forbidden, 40400 not found, 50000 server error, 60000 business failure).
	Code int `json:"code" example:"20000"`
	// Message is the human readable result: "Success" when Code is 20000,
	// otherwise the failure reason that can be shown to the user.
	Message string `json:"message" example:"Success"`
	// Data is the response payload. It is null for failures and for successful
	// endpoints that return nothing.
	Data T `json:"data"`
}

// Paginate is the paginated payload returned inside Response.Data for list
// endpoints.
type Paginate[T any] struct {
	// Total is the number of records matching the filter across all pages.
	Total int64 `json:"total" example:"100"`
	// Page is the returned page number, starting from 1.
	Page int `json:"page" example:"1"`
	// Size is the number of records per page, echoed from the request (default 15).
	Size int `json:"size" example:"15"`
	// Data holds the records of the current page; it is an empty array when the
	// page has no record.
	Data []T `json:"data"`
}

// Event is the payload pushed to clients by server-sent event (SSE) endpoints.
type Event[T any] struct {
	// ID is the event identifier, encoded as a string when the primary key is a
	// snowflake ID. Optional: a reconnecting client sends it back in the
	// Last-Event-ID request header.
	ID any `json:"id,omitempty" swaggertype:"string"`
	// Event is the event name the client must listen to, e.g. "message" or
	// "notice".
	Event string `json:"event" example:"message"`
	// Data is the event payload; its shape depends on Event.
	Data T `json:"data"`
	// Timestamp is the send time in RFC3339. Optional: only present when the
	// producer sets it.
	Timestamp string `json:"timestamp,omitempty" example:"2024-01-01T00:00:00+08:00"`
}
