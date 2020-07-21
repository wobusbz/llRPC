package loadbalance

import (
	"context"

	"github.com/wobusbz/llRPC/regirsty"
)

type Loadbalance interface {
	Name() string
	Select(ctx context.Context, nodes []*regirsty.Node) (node *regirsty.Node, err error)
}