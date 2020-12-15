package api

import (
	"fmt"
	"strings"
)

// File is the struct for the json file
type File struct {
	Checked string      `json:"checked"`
	Updates UpdatesList `json:"updates"`
}

// IsEmpty returns True if File is empty
func (f File) IsEmpty() bool {
	return f.Checked == "" && len(f.Updates) == 0
}

// Copy returns a deep copy of this struct
func (f File) Copy() File {
	cp := File{Checked: f.Checked}
	cp.Updates = f.Updates.Copy()
	return cp
}

// Remove removes update from internal list if name matches
// If newVer isn't an empty string, only remove if that matches as well
func (f *File) Remove(name string, newVer string) bool {
	updates := make(UpdatesList, 0)
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
	updates := make(UpdatesList, 0)
	for _, u := range f.Updates {
		if !strings.Contains(check, u.Pkg) || (newVer && !strings.Contains(check, u.NewVer)) {
			updates = append(updates, u)
		}
	}
	changed := len(f.Updates) != len(updates)
	f.Updates = updates
	return changed
}

func (f *File) String() string {
	ret := fmt.Sprintf("Checked: %s", f.Checked)
	for _, u := range f.Updates {
		ret += "\n" + u.Pkg
		if u.OldVer != "" {
			ret += fmt.Sprintf(" %s", u.OldVer)
		}
		ret += fmt.Sprintf(" -> %s", u.NewVer)
		if u.Repo != "" {
			ret += fmt.Sprintf(" [%s]", u.Repo)
		}
	}
	return ret
}

// UpdatesList is a list of Update with extra methods
type UpdatesList []Update

// Contains returns true if list contains other
func (u *UpdatesList) Contains(other Update) bool {
	for _, ref := range *u {
		if other.Equals(&ref) {
			return true
		}
	}
	return false
}

// Copy returns a deep copy of this list
func (u *UpdatesList) Copy() UpdatesList {
	ret := make(UpdatesList, len(*u))
	copy(ret, *u)
	return ret
}

// Update is the struct for pending updates
type Update struct {
	Pkg    string `json:"pkg"`
	OldVer string `json:"oldVer,omitempty"`
	NewVer string `json:"newVer"`
	Repo   string `json:"repo,omitempty"`
}

// Equals returns true if other update is equal to self
func (u *Update) Equals(other *Update) bool {
	return u.NewVer == other.NewVer && u.OldVer == other.OldVer &&
		u.Pkg == other.Pkg && u.Repo == other.Repo
}
