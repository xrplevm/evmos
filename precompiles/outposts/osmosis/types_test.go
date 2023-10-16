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
		expPass     bool
		errContains string
	}{
		{
			name:     "success - create memo",
			outputDenom: "uosmo",
			receiver: "receiveraddress",
			contract: "xcscontract",
			expPass:  true,
		},
	}

	for _, tc := range testcases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			memo, err := osmosisoutpost.CreateMemo(tc.outputDenom, tc.receiver, tc.contract)
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