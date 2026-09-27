package laserlabel

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/borud/laserlabel/pkg/fonts"
)

type fontsCmd struct {
	Ls fontsLsCmd `kong:"cmd,aliases='list',help='list installed fonts'"`
}

type fontsLsCmd struct {
	Dirs   []string `kong:"help='font directories to scan (default: system font directories)'"`
	Filter string   `kong:"arg,optional,help='only show fonts whose family contains this text'"`
	IDs    bool     `kong:"name='ids',help='show font IDs'"`
}

func (f *fontsLsCmd) Run(_ *Options) error {
	dirs := f.Dirs
	if len(dirs) == 0 {
		dirs = fonts.DefaultDirs()
	}

	filter := strings.ToLower(f.Filter)
	for _, info := range fonts.Discover(dirs, slog.Default()) {
		if !strings.Contains(strings.ToLower(info.Family), filter) {
			continue
		}
		if f.IDs {
			fmt.Printf("%-40s %-20s %s\n", info.Family, info.Style, info.ID)
			continue
		}
		fmt.Printf("%-40s %s\n", info.Family, info.Style)
	}
	return nil
}
