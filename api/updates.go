package api

// File is the struct for the json file
type File struct {
	Checked string   `json:"checked"`
	Updates []Update `json:"updates"`
}

// Update is the struct for pending updates
type Update struct {
	Pkg    string `json:"pkg"`
	OldVer string `json:"oldVer,omitempty"`
	NewVer string `json:"newVer"`
	Repo   string `json:"repo,omitempty"`
}
