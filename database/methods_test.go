package database

import "testing"

func TestStakerTxID(t *testing.T) {
	txID := "tx"
	tests := []struct {
		name string
		tx   PChainTx
		want string
	}{
		{"add permissionless validator", PChainTx{Type: PChainAddPermissionlessValidatorTx, TxID: &txID}, "tx"},
		{"add auto renewed validator (first cycle)", PChainTx{Type: PChainAddAutoRenewedValidatorTx, TxID: &txID}, "tx"},
		{"reward auto renewed validator (renewed cycle)", PChainTx{Type: PChainRewardAutoRenewedValidatorTx, TxID: &txID, RewardTxID: "add"}, "add"},
		{"commit block row", PChainTx{BlockType: PChainCommitBlock}, ""},
	}
	for _, test := range tests {
		if got := test.tx.StakerTxID(); got != test.want {
			t.Errorf("%s: expected staker tx id %q, got %q", test.name, test.want, got)
		}
	}
}
