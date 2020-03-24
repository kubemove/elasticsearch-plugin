package test

import (
	"github.com/appscode/go/crypto/rand"
	"github.com/kubemove/elasticsearch-plugin/pkg/util"
)

type Invocation struct {
	*util.PluginOptions
	testID string
}

func NewInvocation(opt *util.PluginOptions) Invocation {
	return Invocation{
		PluginOptions: opt,
		testID: rand.WithUniqSuffix("e2e"),
	}
}
