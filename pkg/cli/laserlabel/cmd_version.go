package laserlabel

import (
	"fmt"

	"github.com/borud/laserlabel/pkg/appinfo"
)

type versionCmd struct{}

func (v *versionCmd) Run(_ *Options) error {
	fmt.Printf("laserlabel %s (commit %s, built %s by %s)\n",
		appinfo.TaggedVersion, appinfo.CommitHash, appinfo.BuildTime, appinfo.BuiltBy)
	return nil
}
