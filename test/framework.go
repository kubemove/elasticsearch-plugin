package test

import (
	"time"

	"github.com/appscode/go/crypto/rand"
	"github.com/kubemove/elasticsearch-plugin/pkg/util"
)

const (
	DefaultTimeout       = 20 * time.Minute
	DefaultRetryInterval = 2 * time.Second
)

type Invocation struct {
	*util.PluginOptions
	testID string
}

func NewInvocation(opt *util.PluginOptions) Invocation {
	return Invocation{
		PluginOptions: opt,
		testID:        rand.WithUniqSuffix("e2e"),
	}
}
