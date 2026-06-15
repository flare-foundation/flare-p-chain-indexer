package utils

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"flare-indexer/services/api"
	"flare-indexer/utils"
	"fmt"
	"strings"

	"github.com/ava-labs/libevm/accounts/abi"
	"github.com/ava-labs/libevm/common/hexutil"
	"github.com/ava-labs/libevm/crypto"
)

var (
	AbiTypeUint8, _   = abi.NewType("uint8", "uint8", nil)
	AbiTypeUint16, _  = abi.NewType("uint16", "uint16", nil)
	AbiTypeUint32, _  = abi.NewType("uint32", "uint32", nil)
	AbiTypeUint64, _  = abi.NewType("uint64", "uint64", nil)
	AbiTypeBytes20, _ = abi.NewType("bytes20", "bytes20", nil)
	AbiTypeBytes32, _ = abi.NewType("bytes32", "bytes32", nil)
	AbiTypeString, _  = abi.NewType("string", "string", nil)
)

var (
	errEmptyRequestError = errors.New("request is empty")
)

func HashPChainStaking(request *api.ARPChainStaking, response *api.DHPChainStaking, salt string) (string, error) {
	arguments := abi.Arguments{
		abi.Argument{Type: AbiTypeUint16},  // AttestationType
		abi.Argument{Type: AbiTypeUint32},  // SourceId
		abi.Argument{Type: AbiTypeUint32},  // BlockNumber
		abi.Argument{Type: AbiTypeBytes32}, // TransactionHash
		abi.Argument{Type: AbiTypeUint8},   // TransactionType
		abi.Argument{Type: AbiTypeBytes20}, // NodeId
		abi.Argument{Type: AbiTypeUint64},  // startTime
		abi.Argument{Type: AbiTypeUint64},  // endTime
		abi.Argument{Type: AbiTypeUint64},  // weight
		abi.Argument{Type: AbiTypeBytes20}, // sourceAddress
	}
	txHash32, err := utils.TransactionHexToBytes32(response.TransactionHash)
	if err != nil {
		return "", err
	}
	nodeId20, err := utils.Hex20ToBytes20(response.NodeId)
	if err != nil {
		return "", err
	}
	srcAddress, err := utils.Hex20ToBytes20(response.SourceAddress)
	if err != nil {
		return "", err
	}
	values := []interface{}{
		request.AttestationType,
		uint32(request.SourceId),
		response.BlockNumber,
		txHash32,
		response.TransactionType,
		nodeId20,
		uint64(response.StartTime),
		uint64(response.EndTime),
		response.Weight,
		srcAddress,
	}
	if len(salt) > 0 {
		arguments = append(arguments, abi.Argument{Type: AbiTypeString}) // salt
		values = append(values, salt)
	}
	result, err := arguments.Pack(values...)
	if err != nil {
		return "", err
	}
	h := crypto.Keccak256Hash(result)
	return h.Hex(), nil
}

func PackPChainStakingRequest(request *api.ARPChainStaking) (string, error) {
	if request == nil {
		return "", errEmptyRequestError
	}
	miCode, err := utils.PadHexString(request.MessageIntegrityCode, 64)
	if err != nil {
		return "", fmt.Errorf("error packing MessageIntegrityCode: %w", err)
	}
	id, err := utils.PadHexString(request.Id, 64)
	if err != nil {
		return "", fmt.Errorf("error packing id: %w", err)
	}
	response := "0x"
	response += utils.UInt16ToHex(uint16(request.AttestationType))
	response += utils.UInt32ToHex(uint32(request.SourceId))
	response += miCode
	response += id
	response += utils.UInt32ToHex(request.BlockNumber)
	return response, nil
}

func UnpackPChainStakingRequest(request string) (*api.ARPChainStaking, error) {
	request = strings.TrimPrefix(request, "0x")
	byteRequest, err := hex.DecodeString(request)
	if err != nil {
		return nil, fmt.Errorf("error decoding request: %w", err)
	}
	if len(byteRequest) != 74 {
		return nil, fmt.Errorf("invalid request length")
	}
	result := api.ARPChainStaking{}
	result.AttestationType = api.AttestationType(binary.LittleEndian.Uint16(byteRequest[0:2]))
	result.SourceId = api.SourceId(binary.LittleEndian.Uint32(byteRequest[2:6]))
	result.MessageIntegrityCode = hexutil.Encode(byteRequest[6:38])
	result.Id = hexutil.Encode(byteRequest[38:70])
	result.BlockNumber = binary.LittleEndian.Uint32(byteRequest[70:74])
	return &result, nil
}
