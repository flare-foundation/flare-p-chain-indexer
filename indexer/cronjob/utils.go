package cronjob

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/vms/platformvm/api"
	"github.com/ybbus/jsonrpc/v3"
)

const (
	ConnectionTimeout = 3 * time.Second
)

type PermissionedValidators struct {
	Validators []*api.PermissionedValidator
}

// Get connected validators from P-Chain, returns nil on error
// Status is 0 if success, -1 on timeout, -2 on other error
// Error is nil on succes or when rpc call fails in this case status is < 0
func CallPChainGetConnectedValidators(client jsonrpc.RPCClient) ([]*api.PermissionedValidator, int8, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ConnectionTimeout)
	defer cancel()
	response, err := client.Call(ctx, "platform.getCurrentValidators")

	switch err.(type) {
	case nil:
		reply := PermissionedValidators{}
		err = response.GetObject(&reply)
		return reply.Validators, 0, nil
	case *jsonrpc.HTTPError:
		return nil, -2, nil
	default:
		return nil, -1, nil
	}
}
