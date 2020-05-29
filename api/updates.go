package api

import "time"

// File is the struct for the yaml file
type File struct {
	Checked time.Time
	Updates []Update
}

// Update is the struct for pending updates
type Update struct {
	Pkg    string `yaml:"pkg" json:"pkg"`
	OldVer string `yaml:"old_ver,omitempty" json:"oldVer,omitempty"`
	NewVer string `yaml:"new_ver" json:"newVer"`
	Repo   string `yaml:"repo,omitempty" json:"repo,omitempty"`
}
