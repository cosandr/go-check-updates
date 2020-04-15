package types

import "time"

// YamlT is the struct for the yaml file
type YamlT struct {
	Checked time.Time
	Updates []Update
}

// Update is the struct for pending updates
type Update struct {
	Pkg    string
	OldVer string `yaml:",omitempty"`
	NewVer string
	Repo   string `yaml:",omitempty"`
}
