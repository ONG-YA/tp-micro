package hosts

import (
	"bytes"
	"context"
	"encoding/json"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/henrylee2cn/goutil"
	tp "github.com/henrylee2cn/teleport/v6"
	"github.com/henrylee2cn/teleport/v6/codec"
	"github.com/xiaoenai/tp-micro/v6/clientele"
	"github.com/xiaoenai/tp-micro/v6/gateway/sdk"
	"github.com/xiaoenai/tp-micro/v6/gateway/types"
	"github.com/xiaoenai/tp-micro/v6/model/etcd"
)

const (
	// minimum lease TTL is 10-second
	minLeaseTTL      = 10
	maxGatewayAmount = 10
)

var (
	hostsPrefix = "MICRO-GW_HOSTS"
)

// HostsNamespace returns the gateway hosts prefix of ETCD key.
func HostsNamespace() string {
	return hostsPrefix
}

// SetHostsNamespace sets the gateway hosts prefix of ETCD key.
func SetHostsNamespace(prefix string) {
	hostsPrefix = prefix
}

// Hosts gateway ip:port list
type Hosts struct {
	serviceKey      string
	outerSocketAddr string
	innerSocketAddr string
	webSocketAddr   string
	ips             atomic.Value
	ipsLock         sync.Mutex
	weightIps       map[string]*WeightIp
	weightIpsLock   sync.Mutex
	leaseid         etcd.LeaseID
}

var (
	hosts = new(Hosts)
)

// Start initializes and starts the hosts program.
func Start(httpAddr, outerSocketAddr, innerSocketAddr, webSocketAddr string) error {
	hosts.init(httpAddr, outerSocketAddr, innerSocketAddr, webSocketAddr)
	return hosts.start()
}

// GwHosts returns the gateway ip list.
func GwHosts() *types.GwHosts {
	r, _ := hosts.ips.Load().(*types.GwHosts)
	return r
}

// SocketAddress returns the current gateway addresses.
func SocketAddress() (outer, inner string) {
	return hosts.outerSocketAddr, hosts.innerSocketAddr
}

func (h *Hosts) init(httpAddr, outerSocketAddr, innerSocketAddr, webSocketAddr string) {
	h.outerSocketAddr = outerSocketAddr
	h.innerSocketAddr = innerSocketAddr
	h.webSocketAddr = webSocketAddr
	h.serviceKey = hostsPrefix + "@" + httpAddr + "@" + outerSocketAddr + "@" + innerSocketAddr + "@" + webSocketAddr
}

// start starts the SocketHosts program.
func (h *Hosts) start() error {
	h.ips.Store(new(types.GwHosts))
	ch, err := h.keepAlive()
	if err != nil {
		return err
	}
	go func() {
		for {
			select {
			case <-clientele.GetEtcdClient().Ctx().Done():
				tp.Warnf("[GW_HOSTS] stop!")
				h.revoke()
				return
			case _, ok := <-ch:
				if !ok {
					tp.Debugf("[GW_HOSTS] etcd keep alive channel closed, and restart it")
					h.revoke()
					ch = h.anywayKeepAlive()
				}
			}
		}
	}()
	h.resetGatewayIps(true)
	go h.watchEtcd()
	return nil
}

func (h *Hosts) anywayKeepAlive() <-chan *etcd.LeaseKeepAliveResponse {
	ch, err := h.keepAlive()
	for err != nil {
		time.Sleep(minLeaseTTL * time.Second)
		ch, err = h.keepAlive()
	}
	return ch
}

func (h *Hosts) keepAlive() (<-chan *etcd.LeaseKeepAliveResponse, error) {
	resp, err := clientele.GetEtcdClient().Grant(context.TODO(), minLeaseTTL)
	if err != nil {
		return nil, err
	}
	_, err = clientele.GetEtcdClient().Put(
		context.TODO(),
		h.serviceKey,
		"",
		etcd.WithLease(resp.ID),
	)
	if err != nil {
		return nil, err
	}
	ch, err := clientele.GetEtcdClient().KeepAlive(context.TODO(), resp.ID)
	if err != nil {
		return nil, err
	}
	h.leaseid = resp.ID
	return ch, nil
}

func (h *Hosts) revoke() {
	_, err := clientele.GetEtcdClient().Revoke(context.TODO(), h.leaseid)
	if err != nil {
		tp.Errorf("[GW_HOSTS] revoke host error: %s", err.Error())
		return
	}
}

func (h *Hosts) watchEtcd() {
	const (
		interval = minLeaseTTL * 2 * time.Second
	)
	var (
		updateCh = make(chan struct{})
	)
	go func() {
		var ticker = time.NewTicker(interval)
		for {
			select {
			case <-updateCh:
				ticker.Stop()
				h.weightIpsLock.Lock()
				h.sortAndStoreIpsLocked()
				h.weightIpsLock.Unlock()
				ticker = time.NewTicker(interval)
			case <-ticker.C:
				h.weightIpsLock.Lock()
				h.sortAndStoreIpsLocked()
				h.weightIpsLock.Unlock()
			}
		}
	}()

	var (
		key             string
		httpAddr        string
		outerSocketAddr string
		innerSocketAddr string
		webSocketAddr   string
		ok              bool
		watcher         = clientele.GetEtcdClient().Watch(
			context.Background(),
			hostsPrefix,
			etcd.WithPrefix(),
		)
	)
	for wresp := range watcher {
		for _, ev := range wresp.Events {
			httpAddr, outerSocketAddr, innerSocketAddr, webSocketAddr, ok = splitHostsKey(ev.Kv.Key)
			if !ok {
				continue
			}
			key = string(ev.Kv.Key)
			h.weightIpsLock.Lock()
			_, ok = h.weightIps[key]

			switch ev.Type {
			case etcd.EventTypePut:
				if !ok {
					h.weightIps[key] = &WeightIp{
						httpAddr:        httpAddr,
						outerSocketAddr: outerSocketAddr,
						innerSocketAddr: innerSocketAddr,
						webSocketAddr:   webSocketAddr,
					}
					tp.Infof("[GW_HOSTS] add host: %s", key)
				}

			case etcd.EventTypeDelete:
				if ok {
					delete(h.weightIps, key)
					tp.Infof("[GW_HOSTS] delete host: %s", key)
					select {
					case updateCh <- struct{}{}:
					default:
					}
				}
			}

			h.weightIpsLock.Unlock()
		}
	}
}

func (h *Hosts) resetGatewayIps(goSort bool) {
	resp, err := clientele.GetEtcdClient().Get(
		context.Background(),
		hostsPrefix,
		etcd.WithPrefix(),
		etcd.WithSort(etcd.SortByKey, etcd.SortDescend),
	)
	if err != nil || resp.Kvs == nil || len(resp.Kvs) == 0 {
		h.ips.Store(new(types.GwHosts))
		return
	}
	var (
		httpAddr        string
		outerSocketAddr string
		innerSocketAddr string
		webSocketAddr   string
		ok              bool
		m               = make(map[string]*WeightIp, len(resp.Kvs))
	)
	for _, n := range resp.Kvs {
		httpAddr, outerSocketAddr, innerSocketAddr, webSocketAddr, ok = splitHostsKey(n.Key)
		if !ok {
			tp.Warnf("[GW_HOSTS] invalid host key: %s", n.Key)
			continue
		}
		m[string(n.Key)] = &WeightIp{
			httpAddr:        httpAddr,
			outerSocketAddr: outerSocketAddr,
			innerSocketAddr: innerSocketAddr,
			webSocketAddr:   webSocketAddr,
		}
	}
	if goSort {
		go func() {
			time.Sleep(1e9)
			h.weightIpsLock.Lock()
			h.weightIps = m
			h.sortAndStoreIpsLocked()
			h.weightIpsLock.Unlock()
		}()
	} else {
		h.weightIpsLock.Lock()
		h.weightIps = m
		h.sortAndStoreIpsLocked()
		h.weightIpsLock.Unlock()
	}
}

func (h *Hosts) sortAndStoreIpsLocked() {
	cnt := len(h.weightIps)
	if cnt == 0 {
		return
	}
	var (
		reply   *types.SocketTotalReply
		t       time.Time
		sortIps = make(SortWeightIps, 0, cnt)
		stat    *tp.Status
	)
	for _, w := range h.weightIps {
		if len(w.innerSocketAddr) > 0 {
			t = time.Now()
			reply, stat = sdk.SocketTotal(
				w.innerSocketAddr,
				tp.WithBodyCodec(codec.ID_PROTOBUF),
			)
			if stat != nil {
				tp.Warnf("[GW_HOSTS] not available host: innerSocketAddr: %s, error: %s", w.innerSocketAddr, stat)
				continue
			}
			w.weight = -int64(reply.ConnTotal) - int64(time.Since(t)/time.Millisecond)
		}
		sortIps = append(sortIps, w)
	}
	sort.Sort(sortIps)
	ips := &types.GwHosts{
		Http:   make([]string, 0, len(sortIps)),
		Socket: make([]string, 0, len(sortIps)),
		Ws:     make([]string, 0, len(sortIps)),
	}
	var (
		// Eliminate duplicates
		httpMap   = make(map[string]bool, len(sortIps))
		socketMap = make(map[string]bool, len(sortIps))
		wsMap     = make(map[string]bool, len(sortIps))
	)
	for _, w := range sortIps {
		if len(w.httpAddr) > 0 && len(ips.Http) < maxGatewayAmount {
			if !httpMap[w.httpAddr] {
				ips.Http = append(ips.Http, w.httpAddr)
				httpMap[w.httpAddr] = true
			}
		}
		if len(w.outerSocketAddr) > 0 && len(ips.Socket) < maxGatewayAmount {
			if !socketMap[w.outerSocketAddr] {
				ips.Socket = append(ips.Socket, w.outerSocketAddr)
				socketMap[w.outerSocketAddr] = true
			}
		}
		if len(w.webSocketAddr) > 0 && len(ips.Ws) < maxGatewayAmount {
			if !wsMap[w.webSocketAddr] {
				ips.Ws = append(ips.Ws, w.webSocketAddr)
				wsMap[w.webSocketAddr] = true
			}
		}
	}
	h.ipsLock.Lock()
	h.ips.Store(ips)
	h.ipsLock.Unlock()
	b, _ := json.MarshalIndent(ips, "", "  ")
	tp.Tracef("[GW_HOSTS] update hosts: %s", b)
}

type (
	// SortWeightIps IP weight information list for sorting
	SortWeightIps []*WeightIp
	// WeightIp IP information with weight
	WeightIp struct {
		httpAddr        string
		outerSocketAddr string
		innerSocketAddr string
		webSocketAddr   string
		weight          int64
	}
)

// Len is the number of elements in the collection.
func (s SortWeightIps) Len() int {
	return len(s)
}

// Less reports whether the element with
// index i should sort before the element with index j.
func (s SortWeightIps) Less(i, j int) bool {
	return s[i].weight > s[j].weight
}

// Swap swaps the elements with indexes i and j.
func (s SortWeightIps) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func splitHostsKey(valueBytes []byte) (httpAddr, outerSocketAddr, innerSocketAddr, webSocketAddr string, ok bool) {
	valueBytes = bytes.TrimPrefix(valueBytes, goutil.StringToBytes(hostsPrefix))
	a := bytes.Split(valueBytes, []byte{'@'})
	if len(a) != 5 {
		return
	}
	httpAddr = string(a[1])
	outerSocketAddr = string(a[2])
	innerSocketAddr = string(a[3])
	webSocketAddr = string(a[4])
	ok = true
	return
}
