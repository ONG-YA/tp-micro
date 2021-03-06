// Code generated by 'micro gen' command.
// DO NOT EDIT!

package sdk

import (
	"fmt"

	"github.com/henrylee2cn/erpc/v6"
	micro "github.com/xiaoenai/tp-micro/v6"
	"github.com/xiaoenai/tp-micro/v6/discovery"
	"github.com/xiaoenai/tp-micro/v6/model/etcd"

	"github.com/xiaoenai/tp-micro/v6/examples/project/args"
)

var _ = fmt.Sprintf
var client *micro.Client

// Init initializes client with configs.
func Init(cliConfig micro.CliConfig, etcdConfing etcd.EasyConfig) {
	client = micro.NewClient(
		cliConfig,
		discovery.NewLinker(etcdConfing),
	)
}

// InitWithClient initializes client with specified object.
func InitWithClient(cli *micro.Client) {
	client = cli
}

// Stat handler
func Stat(arg *args.StatArg, setting ...erpc.MessageSetting) *erpc.Status {
	setting = append(setting, erpc.WithAddMeta("ts", fmt.Sprintf("%v", arg.Ts)))
	return client.Push("/project/stat", arg, setting...)
}

// Home handler
func Home(arg *args.EmptyStruct, setting ...erpc.MessageSetting) (*args.HomeResult, *erpc.Status) {
	result := new(args.HomeResult)
	status := client.Call("/project/home", arg, result, setting...).Status()
	return result, status
}

// Divide handler
func Math_Divide(arg *args.DivideArg, setting ...erpc.MessageSetting) (*args.DivideResult, *erpc.Status) {
	result := new(args.DivideResult)
	status := client.Call("/project/math/divide", arg, result, setting...).Status()
	return result, status
}
