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
			name:     "fail - input and outp cannot be the same",
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