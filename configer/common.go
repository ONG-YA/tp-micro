package configer

import (
	"sync"

	"github.com/henrylee2cn/cfgo"
	"github.com/henrylee2cn/erpc/v6"
	"github.com/xiaoenai/tp-micro/v6/model/etcd"
)

// Config config interface
type Config interface {
	UnmarshalJSON([]byte) error
	MarshalJSON() ([]byte, error)
	Reload([]byte) error
}

const (
	// KEY_PREFIX the prifix of config data key in etcd
	KEY_PREFIX = "MICRO-CONF"
)

// NewKey creates a config data key
// Note: service and version can not contant "@"!
func NewKey(service string, version string) string {
	return KEY_PREFIX + "@" + service + "@" + version
}

func must(err error) {
	if err != nil {
		erpc.Fatalf("%v", err)
	}
}

var (
	onceRegYaml    sync.Once
	etcdEasyConfig *etcd.EasyConfig
)

type etcdConfig etcd.EasyConfig

func (e *etcdConfig) Reload(bindFunc cfgo.BindFunc) error {
	return bindFunc()
}

// NewEtcdClientFromYaml uses config/etcd.yaml to create a etcd client.
func NewEtcdClientFromYaml() (*etcd.Client, error) {
	onceRegYaml.Do(func() {
		etcdEasyConfig = new(etcd.EasyConfig)
		cfgo.
			MustGet("./config/etcd.yaml").
			MustReg("ETCD", (*etcdConfig)(etcdEasyConfig))
	})
	return etcd.EasyNew(*etcdEasyConfig)
}
