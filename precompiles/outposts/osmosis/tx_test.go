// Copyright Tharsis Labs Ltd.(Evmos)
// SPDX-License-Identifier:ENCL-1.0(https://github.com/evmos/evmos/blob/main/LICENSE)

package osmosis_test

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/core/vm"
	cmn "github.com/evmos/evmos/v14/precompiles/common"
	"github.com/evmos/evmos/v14/precompiles/outposts/osmosis"
)

func (s *PrecompileTestSuite) TestSwap() {
	method := s.precompile.Methods[osmosis.SwapMethod]

	testCases := []struct {
		name string
		malleate func() []interface{}
		postCheck func()
		gas uint64
		ExpError bool
		errContains string
	} {
		{
			"fail - empty input args",
			func() []interface{} {
				return []interface{}{}
			},
			func() {},
			200000,
			true,
			fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 4, 0),
		},
	}

	for _, tc := range testCases {
		s.Run(
			tc.name,
			func() {
				s.SetupTest()

				contract := vm.NewContract(vm.AccountRef(s.address), s.precompile, big.NewInt(0), tc.gas)

				_, err := s.precompile.Swap(s.ctx, s.address, contract, s.stateDB, &method, tc.malleate())
				if tc.ExpError {
					s.Require().ErrorContains(err, tc.errContains)
				} else {
					s.Require().NoError(err)
					tc.postCheck()	
				}
			})
	}
}
