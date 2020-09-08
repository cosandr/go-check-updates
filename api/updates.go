package api

// File is the struct for the json file
type File struct {
	Checked string   `json:"checked"`
	Updates []Update `json:"updates"`
}

// IsEmpty returns True if File is empty
func (f File) IsEmpty() bool {
	return f.Checked == "" && len(f.Updates) == 0
}

// Copy returns a deep copy of this struct
func (f File) Copy() File {
	cp := File{Checked: f.Checked}
	cp.Updates = make([]Update, len(f.Updates))
	copy(cp.Updates, f.Updates)
	return cp
}

// Update is the struct for pending updates
type Update struct {
	Pkg    string `json:"pkg"`
	OldVer string `json:"oldVer,omitempty"`
	NewVer string `json:"newVer"`
	Repo   string `json:"repo,omitempty"`
}
