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

// RemoveIfNew removes update from internal list if name and new version match
func (f *File) RemoveIfNew(name string, newVer string) bool {
	updates := make([]Update, 0)
	for _, u := range f.Updates {
		if u.Pkg != name || u.NewVer != newVer {
			updates = append(updates, u)
		}
	}
	changed := len(f.Updates) != len(updates)
	f.Updates = updates
	return changed
}

// Update is the struct for pending updates
type Update struct {
	Pkg    string `json:"pkg"`
	OldVer string `json:"oldVer,omitempty"`
	NewVer string `json:"newVer"`
	Repo   string `json:"repo,omitempty"`
}
