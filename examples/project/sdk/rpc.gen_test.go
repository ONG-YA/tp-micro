// Code generated by 'micro gen' command.
// DO NOT EDIT!

package sdk_test

import (
	"encoding/json"
	"fmt"

	"github.com/henrylee2cn/erpc/v6"
	micro "github.com/xiaoenai/tp-micro/v6"
	"github.com/xiaoenai/tp-micro/v6/model/etcd"

	"github.com/xiaoenai/tp-micro/v6/examples/project/args"
	"github.com/xiaoenai/tp-micro/v6/examples/project/sdk"
)

func init() {
	sdk.Init(
		micro.CliConfig{
			Failover:        3,
			HeartbeatSecond: 4,
		},
		etcd.EasyConfig{
			Endpoints: []string{"http://127.0.0.1:2379"},
		},
	)
}

func toJsonBytes(i interface{}) []byte {
	b, _ := json.MarshalIndent(i, "", "  ")
	return b
}

func ExampleStat() {
	status := sdk.Stat(&args.StatArg{})
	if status != nil {
		erpc.Errorf("Stat: status: %s", toJsonBytes(status))
	}
	fmt.Printf("")
	// Output:
}

func ExampleHome() {
	result, status := sdk.Home(&args.EmptyStruct{})
	if status != nil {
		erpc.Errorf("Home: status: %s", toJsonBytes(status))
	} else {
		erpc.Infof("Home: result: %s", toJsonBytes(result))
	}
	fmt.Printf("")
	// Output:
}

func ExampleMath_Divide() {
	result, status := sdk.Math_Divide(&args.DivideArg{})
	if status != nil {
		erpc.Errorf("Math_Divide: status: %s", toJsonBytes(status))
	} else {
		erpc.Infof("Math_Divide: result: %s", toJsonBytes(result))
	}
	fmt.Printf("")
	// Output:
}
