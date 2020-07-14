package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"time"

	"github.com/wobusbz/llRPC/regirsty"
	"go.etcd.io/etcd/clientv3"
)

type EtcdRegistry struct {
	options *regirsty.Options
	clinet  *clientv3.Client
	service chan *regirsty.Service

	registerServiceMap map[string]*RegisterService
}

type RegisterService struct {
	id      clientv3.LeaseID
	service *regirsty.Service

	registerd bool
	keepChan  <-chan *clientv3.LeaseKeepAliveResponse
}

var (
	etcdRegistry = &EtcdRegistry{
		service:            make(chan *regirsty.Service, 8),
		registerServiceMap: make(map[string]*RegisterService, 8),
	}
)

func init() {
	regirsty.RegisteryPlugin(etcdRegistry)
	go etcdRegistry.run()
}

func (e *EtcdRegistry) Name() string {
	return "etcd"
}

func (e *EtcdRegistry) Init(ctx context.Context, opts ...regirsty.Option) (err error) {
	e.options = &regirsty.Options{}
	for _, opt := range opts {
		opt(e.options)
	}

	e.clinet, err = clientv3.New(
		clientv3.Config{
			Endpoints:   e.options.Addrs,
			DialTimeout: e.options.Timeout,
		})
	if err != nil {
		return
	}
	return
}

func (e *EtcdRegistry) Register(ctx context.Context, service *regirsty.Service) (err error) {
	select {
	case e.service <- service:
	default:
		return fmt.Errorf("register plugin is full")
	}
	return
}

func (e *EtcdRegistry) Unregister(ctx context.Context, service *regirsty.Service) (err error) {
	return
}

func (e *EtcdRegistry) run() {
	for {
		select {
		case service := <-e.service:
			if _, ok := e.registerServiceMap[service.Name]; ok {
				break
			}
			e.registerServiceMap[service.Name] = &RegisterService{
				service: service,
			}
		default:
			e.registryOrKeepAlive()
			time.Sleep(time.Microsecond * 500)
		}
	}
}

func (e *EtcdRegistry) registryOrKeepAlive() {
	for _, service := range e.registerServiceMap {
		if service.registerd {
			e.keepAlive(service)
			continue
		}
		e.registryService(service)
	}
}

func (e *EtcdRegistry) registryService(service *RegisterService) (err error) {
	resp, err := e.clinet.Grant(context.TODO(), e.options.HeartBeat)
	if err != nil {
		return
	}
	service.id = resp.ID
	for _, node := range service.service.Nodes {
		temp := e.servicePath(&regirsty.Service{
			Name:  service.service.Name,
			Nodes: []*regirsty.Node{node},
		})
		data, err := json.Marshal(temp)
		if err != nil {
			continue
		}
		if _, err = e.clinet.Put(context.TODO(), temp, string(data), clientv3.WithLease(resp.ID)); err != nil {
			continue
		}
		ch, err := e.clinet.KeepAlive(context.TODO(), resp.ID)
		if err != nil {
			continue
		}
		service.keepChan = ch
		service.registerd = true
	}
	return
}

func (e *EtcdRegistry) keepAlive(service *RegisterService) {
	select {
	case resp := <-service.keepChan:
		if resp == nil {
			service.registerd = false
			return
		}
	}
}

func (e *EtcdRegistry) servicePath(service *regirsty.Service) string {
	return path.Join(e.options.RegistryPath, service.Name, fmt.Sprintf("%s:%d", service.Nodes[0].Ip, service.Nodes[0].Port))
}
