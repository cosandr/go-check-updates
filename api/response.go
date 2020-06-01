package api

// Response is what is sent back to the client from a GET request
type Response struct {
	Data     *File  `json:"data,omitempty"`
	Error    string `json:"error,omitempty"`
	FilePath string `json:"filePath,omitempty"`
	Queued   *bool  `json:"queued,omitempty"`
}
