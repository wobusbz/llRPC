package loadbalance

import (
	"context"
	"errors"
	"math/rand"

	"github.com/wobusbz/llRPC/regirsty"
)

type RandomBalance struct {
}

func (r *RandomBalance) Name() string {
	return "random"
}

func (r *RandomBalance) Select(ctx context.Context, nodes []*regirsty.Node) (node *regirsty.Node, err error) {
	if len(nodes) == 0 {
		err = errors.New("node is not fond")
		return
	}

	var sumWeight int

	for _, weigHt := range nodes {
		if weigHt.WeigHt == 0{
			weigHt.WeigHt = 100
		}
		sumWeight += weigHt.WeigHt
	}

	curWeight := rand.Intn(sumWeight)
	curIndex := -1
	for index, node := range nodes {
		curWeight -= node.WeigHt
		if curWeight < 0 {
			curIndex = index
			break
		}
	}

	if curIndex == -1 {
		err = errors.New("node is not fond")
		return
	}
	node = nodes[curIndex]
	return
}
