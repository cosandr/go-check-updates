package api

// Response is what is sent back to the client from a GET request
type Response struct {
	Error    string   `json:"error,omitempty"`
	FilePath string   `json:"filePath,omitempty"`
	Checked  string   `json:"checked,omitempty"`
	Updates  []Update `json:"updates,omitempty"`
}
