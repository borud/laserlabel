package model

// FontInfo describes a font face found on the system.
type FontInfo struct {
	// ID is stable across runs: the file path plus face index.
	ID     string `json:"id"`
	Family string `json:"family"`
	Style  string `json:"style"`
	Path   string `json:"path"`
	Index  int    `json:"index"`
}
