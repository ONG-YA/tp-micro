// Code generated by 'micro gen' command.
// DO NOT EDIT!

package api

import (
	"github.com/henrylee2cn/erpc/v6"

	"github.com/xiaoenai/tp-micro/v6/examples/project/args"
	"github.com/xiaoenai/tp-micro/v6/examples/project/logic"
)

// Stat handler
func Stat(ctx erpc.PushCtx, arg *args.StatArg) *erpc.Status {
	return logic.Stat(ctx, arg)
}
