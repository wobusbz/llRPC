package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wobusbz/llRPC/regirsty"
	"go.etcd.io/etcd/clientv3"
)

const (
	MaxServiceNum          = 8
	MaxSyncServiceInterval = 10 * time.Second
)

type EtcdRegistry struct {
	options            *regirsty.Options
	clinet             *clientv3.Client
	service            chan *regirsty.Service
	registerServiceMap map[string]*RegisterService
	value              atomic.Value
	me                 sync.Mutex
}

type AllServiceInfo struct {
	serviceMap map[string]*regirsty.Service
}

type RegisterService struct {
	id      clientv3.LeaseID
	service *regirsty.Service

	registerd bool
	keepChan  <-chan *clientv3.LeaseKeepAliveResponse
}

var (
	etcdRegistry = &EtcdRegistry{
		service:            make(chan *regirsty.Service, MaxServiceNum),
		registerServiceMap: make(map[string]*RegisterService, MaxServiceNum),
	}
)

func init() {
	allServiceInfo := AllServiceInfo{
		serviceMap: make(map[string]*regirsty.Service),
	}
	etcdRegistry.value.Store(allServiceInfo)
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
	ticker := time.NewTicker(MaxSyncServiceInterval)
	for {
		select {
		case service := <-e.service:
			if registryService, ok := e.registerServiceMap[service.Name]; ok {
				for _, node := range service.Nodes{
					registryService.service.Nodes = append(registryService.service.Nodes, node)
				}
				registryService.registerd = false
				break
			}
			e.registerServiceMap[service.Name] = &RegisterService{
				service: service,
			}
		case <-ticker.C:
			e.syncServiceFromEtcd()
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
		temp := e.serviceNodePath(&regirsty.Service{
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

func (e *EtcdRegistry) serviceNodePath(service *regirsty.Service) string {
	return path.Join(e.options.RegistryPath, service.Name, fmt.Sprintf("%s:%d", service.Nodes[0].Ip, service.Nodes[0].Port))
}

func (e *EtcdRegistry) servicePath(name string) string {
	return path.Join(e.options.RegistryPath, name)
}

func (e *EtcdRegistry) GetService(ctx context.Context, name string) (service *regirsty.Service, err error) {
	service, ok := e.getServiceFromCache(ctx, name)
	if ok {
		return
	}

	e.me.Lock()
	defer e.me.Unlock()
	service, ok = e.getServiceFromCache(ctx, name)
	if ok {
		return
	}
	resp, err := e.clinet.Get(ctx, e.servicePath(name), clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	service = &regirsty.Service{
		Name: name,
	}
	for _, kv := range resp.Kvs {
		value := kv.Value
		var tempService regirsty.Service
		if err = json.Unmarshal(value, &tempService); err != nil {
			return
		}
		for _, node := range tempService.Nodes {
			service.Nodes = append(service.Nodes, node)
		}
	}
	allServiceInfoOld := e.value.Load().(*AllServiceInfo)
	allServiceInfoNew := &AllServiceInfo{
		serviceMap: make(map[string]*regirsty.Service, MaxServiceNum),
	}

	for key, val := range allServiceInfoOld.serviceMap {
		allServiceInfoNew.serviceMap[key] = val
	}
	allServiceInfoNew.serviceMap[name] = service
	e.value.Store(allServiceInfoNew)
	return
}

func (e *EtcdRegistry) getServiceFromCache(ctx context.Context, name string) (service *regirsty.Service, ok bool) {
	allServiceInfo := e.value.Load().(*AllServiceInfo)
	service, ok = allServiceInfo.serviceMap[name]
	if ok {
		return
	}
	return
}

func (e *EtcdRegistry) syncServiceFromEtcd() {
	allServiceInfo := e.value.Load().(*AllServiceInfo)
	var ctx = context.Background()
	allServiceInfoNew := &AllServiceInfo{
		serviceMap: make(map[string]*regirsty.Service, MaxServiceNum),
	}

	for _, service := range allServiceInfo.serviceMap {
		resp, err := e.clinet.Get(ctx, e.servicePath(service.Name), clientv3.WithPrefix())
		if err != nil {
			allServiceInfoNew.serviceMap[service.Name] = service
			continue
		}

		serviceNew := &regirsty.Service{
			Name: service.Name,
		}
		for _, kv := range resp.Kvs {
			value := kv.Value
			var tempService regirsty.Service
			if err = json.Unmarshal(value, &tempService); err != nil {
				return
			}

			for _, node := range tempService.Nodes {
				serviceNew.Nodes = append(serviceNew.Nodes, node)
			}
		}
		allServiceInfoNew.serviceMap[service.Name] = serviceNew
	}
	e.value.Store(allServiceInfoNew)
}
