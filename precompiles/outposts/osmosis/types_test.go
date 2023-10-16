package osmosis_test

import (
	"testing"

	osmosisoutpost "github.com/evmos/evmos/v14/precompiles/outposts/osmosis"
	"github.com/stretchr/testify/require"
)

func TestCreateMemo(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		name        string
		outputDenom string
		receiver    string
		contract	string
		slippage_percentage string
		window_seconds uint64
		expPass     bool
		errContains string
	}{
		{
			name:     "success - create memo",
			outputDenom: "uosmo",
			receiver: "receiveraddress",
			contract: "xcscontract",
			slippage_percentage: "5",
			window_seconds: 10,
			expPass:  true,
		},
	}

	for _, tc := range testcases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			memo, err := osmosisoutpost.CreateMemo(tc.outputDenom, tc.receiver, tc.contract, tc.slippage_percentage, tc.window_seconds)
			if tc.expPass {
				require.NoError(t, err, "expected no error while creating memo")
				require.NotEmpty(t, memo, "expected memo not to be empty")
			} else {
				require.Error(t, err, "expected error while creating memo")
				require.Contains(t, err.Error(), tc.errContains, "expected different error")
			}
		})
	}
}

func TestValidateSwap(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		name string
		portID string
		channelID string
		input string
		output string
		stakingDenom string
		slippagePercentage uint64
		windowSeconds uint64
		expPass     bool
		errContains string
	}{
		{
			name:     "fail - input and output cannot be the same",
			portID: "transfer",
			channelID: "channel-0",
			input: "aevmos",
			output: "aevmos",
			stakingDenom: "aevmos",
			slippagePercentage: osmosisoutpost.DefaultSlippagePercentage,
			windowSeconds: osmosisoutpost.DefaultWindowSeconds,
			expPass:  false,
			errContains: "input and output token cannot be the same",
		},
		{
			name:     "fail - not allowed input",
			portID: "transfer",
			channelID: "channel-0",
			input: "eth",
			output: "uosmo",
			stakingDenom: "aevmos",
			slippagePercentage: osmosisoutpost.DefaultSlippagePercentage,
			windowSeconds: osmosisoutpost.DefaultWindowSeconds,
			expPass:  false,
			errContains: "input not supported",
		},
		{
			name:     "fail - over max slippage percentage",
			portID: "transfer",
			channelID: "channel-0",
			input: "aevmos",
			output: "uosmo",
			stakingDenom: "aevmos",
			slippagePercentage: osmosisoutpost.MaxSlippagePercentage + 1,
			windowSeconds: osmosisoutpost.DefaultWindowSeconds,
			expPass:  false,
			errContains: "slippage percentage",
		},
		{
			name:     "fail - over max window seconds",
			portID: "transfer",
			channelID: "channel-0",
			input: "aevmos",
			output: "uosmo",
			stakingDenom: "aevmos",
			slippagePercentage: osmosisoutpost.DefaultSlippagePercentage,
			windowSeconds: osmosisoutpost.MaxWindowSeconds + 1,
			expPass:  false,
			errContains: "window seconds",
		},
		{
			name:     "pass - correct inputs",
			portID: "transfer",
			channelID: "channel-0",
			input: "aevmos",
			output: "uosmo",
			stakingDenom: "aevmos",
			slippagePercentage: osmosisoutpost.DefaultSlippagePercentage,
			windowSeconds: osmosisoutpost.DefaultWindowSeconds,
			expPass:  true,
			errContains: "",
		},
	}

	for _, tc := range testcases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := osmosisoutpost.ValidateSwap(tc.portID, tc.channelID, tc.input, tc.output, tc.stakingDenom, tc.slippagePercentage, tc.windowSeconds)
			if tc.expPass {
				require.NoError(t, err, "expected no error while creating memo")
				require.NotEmpty(t, "expected memo not to be empty")
			} else {
				require.Error(t, err, "expected error while creating memo")
				require.Contains(t, err.Error(), tc.errContains, "expected different error")
			}
		})
	}	
}