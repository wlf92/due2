/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/13 12:32 上午
 * @Desc: TODO
 */

package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"go.etcd.io/etcd/client/v3"
	"sync"

	"github.com/dobyte/due/registry"
)

var _ registry.Registry = &Registry{}

type Registry struct {
	err        error
	ctx        context.Context
	cancel     context.CancelFunc
	opts       *options
	builtin    bool
	watchers   sync.Map
	registrars sync.Map
}

func NewRegistry(opts ...Option) *Registry {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	r := &Registry{}
	r.opts = o
	r.ctx, r.cancel = context.WithCancel(o.ctx)

	if o.client == nil {
		r.builtin = true
		o.client, r.err = clientv3.New(clientv3.Config{
			Endpoints:   o.addrs,
			DialTimeout: o.dialTimeout,
		})
	}

	return r
}

// Register 注册服务实例
func (r *Registry) Register(ctx context.Context, ins *registry.ServiceInstance) error {
	if r.err != nil {
		return r.err
	}

	v, ok := r.registrars.Load(ins.ID)
	if ok {
		return v.(*registrar).register(ctx, ins)
	}

	reg := newRegistrar(r)

	if err := reg.register(ctx, ins); err != nil {
		return err
	}

	r.registrars.Store(ins.ID, reg)

	return nil
}

// Deregister 解注册服务实例
func (r *Registry) Deregister(ctx context.Context, ins *registry.ServiceInstance) error {
	if r.err != nil {
		return r.err
	}

	v, ok := r.registrars.Load(ins.ID)
	if ok {
		return v.(*registrar).deregister(ctx, ins)
	}

	key := fmt.Sprintf("/%s/%s/%s", r.opts.namespace, ins.Name, ins.ID)
	_, err := r.opts.client.Delete(ctx, key)

	return err
}

// Watch 监听相同服务名的服务实例变化
func (r *Registry) Watch(ctx context.Context, serviceName string) (registry.Watcher, error) {
	if r.err != nil {
		return nil, r.err
	}

	v, ok := r.watchers.Load(serviceName)
	if ok {
		return v.(*watcherMgr).fork(), nil
	}

	w, err := newWatcherMgr(ctx, serviceName, r)
	if err != nil {
		return nil, err
	}
	r.watchers.Store(serviceName, w)

	return w.fork(), nil
}

// Services 获取服务实例列表
func (r *Registry) Services(ctx context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	if r.err != nil {
		return nil, r.err
	}

	v, ok := r.watchers.Load(serviceName)
	if ok {
		return v.(*watcherMgr).services(), nil
	} else {
		return r.services(ctx, serviceName)
	}
}

// Stop 停止注册服务
func (r *Registry) Stop() error {
	if r.err != nil {
		return r.err
	}

	r.cancel()

	if r.builtin {
		return r.opts.client.Close()
	}

	return nil
}

// 获取服务实例列表
func (r *Registry) services(ctx context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	key := fmt.Sprintf("/%s/%s", r.opts.namespace, serviceName)

	res, err := r.opts.client.Get(ctx, key, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	services := make([]*registry.ServiceInstance, 0, len(res.Kvs))
	for _, kv := range res.Kvs {
		service, err := unmarshal(kv.Value)
		if err != nil {
			return nil, err
		}
		services = append(services, service)
	}

	return services, nil
}

func marshal(ins *registry.ServiceInstance) (string, error) {
	buf, err := json.Marshal(ins)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

func unmarshal(data []byte) (*registry.ServiceInstance, error) {
	ins := &registry.ServiceInstance{}
	if err := json.Unmarshal(data, ins); err != nil {
		return nil, err
	}
	return ins, nil
}
