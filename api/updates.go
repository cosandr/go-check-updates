package api

import "strings"

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

// Remove removes update from internal list if name matches
// If newVer isn't an empty string, only remove if that matches as well
func (f *File) Remove(name string, newVer string) bool {
	updates := make([]Update, 0)
	for _, u := range f.Updates {
		if u.Pkg != name || (newVer != "" && u.NewVer != newVer) {
			updates = append(updates, u)
		}
	}
	changed := len(f.Updates) != len(updates)
	f.Updates = updates
	return changed
}

// RemoveContains removes update from internal list if `check` contains `u.Pkg`
// If newVer is true, also check if that is present
func (f *File) RemoveContains(check string, newVer bool) bool {
	updates := make([]Update, 0)
	for _, u := range f.Updates {
		if !strings.Contains(check, u.Pkg) || (newVer && !strings.Contains(check, u.NewVer)) {
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
