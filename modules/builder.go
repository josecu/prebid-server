package modules

import (
	arcspanContextualapp "github.com/prebid/prebid-server/modules/arcspan/contextualapp"
	prebidOrtb2blocking "github.com/prebid/prebid-server/modules/prebid/ortb2blocking"
)

// builders returns mapping between module name and its builder
// vendor and module names are chosen based on the module directory name
func builders() ModuleBuilders {
	return ModuleBuilders{
		"arcspan": {
			"contextualapp": arcspanContextualapp.Builder,
		},
		"prebid": {
			"ortb2blocking": prebidOrtb2blocking.Builder,
		},
	}
}
