package chain

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"flare-indexer/database"
	"flare-indexer/utils"

	"github.com/ava-labs/avalanchego/vms/platformvm/api"
	"github.com/ybbus/jsonrpc/v3"
)

const (
	ConnectionTimeout = 3 * time.Second
)

type ValidatorStatus struct {
	NodeID    string `json:"nodeID"`
	Connected bool   `json:"connected"`
}

type UptimeClient interface {
	GetValidatorStatus() ([]*ValidatorStatus, database.UptimeCronjobStatus, error)
	Now() time.Time
}

type AvalancheUptimeClient struct {
	client jsonrpc.RPCClient
}

func NewAvalancheUptimeClient(endpoint string) UptimeClient {
	return &AvalancheUptimeClient{
		client: jsonrpc.NewClient(endpoint),
	}
}

func (c *AvalancheUptimeClient) GetValidatorStatus() ([]*ValidatorStatus, database.UptimeCronjobStatus, error) {
	validators, status, err := CallPChainGetConnectedValidators(c.client)
	if err != nil {
		return nil, status, err
	}
	vs := make([]*ValidatorStatus, len(validators))
	for i, v := range validators {
		vs[i] = &ValidatorStatus{
			NodeID:    v.NodeID.String(),
			Connected: v.Connected != nil && *v.Connected,
		}
	}
	return vs, status, nil
}

func (c *AvalancheUptimeClient) Now() time.Time {
	return time.Now()
}

type permissionedValidators struct {
	Validators []*api.PermissionedValidator
}

// Get connected validators from P-Chain, returns nil on error
// Status is 0 if success, -1 on timeout, -2 on other error
// Error is nil on succes or when rpc call fails in this case status is < 0
func CallPChainGetConnectedValidators(client jsonrpc.RPCClient) ([]*api.PermissionedValidator, database.UptimeCronjobStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ConnectionTimeout)
	defer cancel()
	response, err := client.Call(ctx, "platform.getCurrentValidators")

	switch err.(type) {
	case nil:
		reply := permissionedValidators{}
		err = response.GetObject(&reply)
		return reply.Validators, database.UptimeCronjobStatusDisconnected, err
	case *jsonrpc.HTTPError:
		return nil, database.UptimeCronjobStatusServiceError, nil
	default:
		return nil, database.UptimeCronjobStatusTimeout, nil
	}
}

type RecordedUptimeData struct {
	NodeID    string `json:"nodeID"`
	Connected int    `json:"connected"`
	Start     int64  `json:"start"`
	End       int64  `json:"end"`
}

type RecordedUptimeClient struct {
	Time *utils.ShiftedTime
	data []RecordedUptimeData
}

func NewRecordedUptimeClient(fileName string, startNow time.Time) (*RecordedUptimeClient, error) {
	data, err := readUptimeRecordings(fileName)
	if err != nil {
		return nil, err
	}

	return &RecordedUptimeClient{
		Time: utils.NewShiftedTime(startNow),
		data: data,
	}, nil
}

func (c *RecordedUptimeClient) GetValidatorStatus() ([]*ValidatorStatus, database.UptimeCronjobStatus, error) {
	now := c.Time.Now().Unix()
	validatorMap := make(map[string]*ValidatorStatus)
	for _, data := range c.data {
		if now >= data.Start && now < data.End {
			if v, ok := validatorMap[data.NodeID]; !ok {
				validatorMap[data.NodeID] = &ValidatorStatus{
					NodeID:    data.NodeID,
					Connected: data.Connected == 1,
				}
			} else {
				// Prefer disconnected over connected
				if data.Connected == 0 {
					v.Connected = false
				}
			}
		}
	}
	return utils.Values(validatorMap), database.UptimeCronjobStatusDisconnected, nil
}

func (c *RecordedUptimeClient) Now() time.Time {
	return c.Time.Now()
}

func (c *RecordedUptimeClient) SetNow(startNow time.Time) {
	c.Time.SetNow(startNow)
}

func (c *RecordedUptimeClient) SetNowUnix(startNow int64) {
	c.Time.SetNowUnix(startNow)
}

func readUptimeRecordings(fileName string) ([]RecordedUptimeData, error) {
	var data []RecordedUptimeData
	jsonFile, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal([]byte(jsonFile), &data)
	if err != nil {
		return nil, err
	}
	return data, nil
}
