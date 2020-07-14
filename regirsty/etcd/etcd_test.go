package etcd

import (
	"context"
	"testing"
	"time"

	"github.com/wobusbz/llRPC/regirsty"
)

func TestEtcdRegistry(t *testing.T) {
	register, err := regirsty.InitRegistery(context.TODO(), "etcd",
		regirsty.WithAddrs([]string{"127.0.0.1:2379"}),
		regirsty.WithTimeout(time.Second),
		regirsty.WithRegistryPath("api/llRPC"),
		regirsty.WithHeartBeat(5),
	)
	if err != nil {
		t.Log(err)
	}
	service := &regirsty.Service{
		Name: "llrpc",
	}

	service.Nodes = append(service.Nodes, &regirsty.Node{
		Ip:   "127.0.0.1",
		Port: 3312,
	})
	register.Register(context.TODO(), service)
	select {}
}
