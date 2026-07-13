package main

import (
	"embed"

	"github.com/BenjiTrapp/ip-to-cloudprovider/provider"
)

// embeddedData holds a build-time snapshot of every provider's IP ranges so the
// tool works out of the box after `go install`, without a prior `update` run.
// Fresh data written to the data directory by `update` always takes precedence.
//
//go:embed */ipranges.json
var embeddedData embed.FS

func init() {
	provider.EmbeddedData = embeddedData
}
