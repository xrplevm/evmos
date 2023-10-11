package osmosis_test

import (
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	cmn "github.com/evmos/evmos/v14/precompiles/common"
	"github.com/evmos/evmos/v14/precompiles/outposts/osmosis"
	"math/big"
)

func (s *PrecompileTestSuite) TestSwap() {
	method := s.precompile.Methods[osmosis.SwapMethod]
	testCases := []struct {
		name        string
		malleate    func(sender, receiver sdk.AccAddress) []interface{}
		postCheck   func(sender, receiver sdk.AccAddress, data []byte, inputArgs []interface{})
		gas         uint64
		expError    bool
		errContains string
	}{
		{
			"fail - empty args",
			func(sender, receiver sdk.AccAddress) []interface{} {
				return []interface{}{}
			},
			func(sender, receiver sdk.AccAddress, data []byte, inputArgs []interface{}) {},
			200000,
			true,
			fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 5, 0),
		},
		{
			"fail - invalid amount",
			func(sender, receiver sdk.AccAddress) []interface{} {
				return []interface{}{
					s.address,
					s.address,
					"test",
					sender.String(),
					receiver.String(),
				}
			},
			func(sender, receiver sdk.AccAddress, data []byte, inputArgs []interface{}) {},
			200000,
			true,
			"invalid amount",
		},
		{
			"fail - invalid bech32 address",
			func(sender, receiver sdk.AccAddress) []interface{} {
				path := NewTransferPath(s.chainA, s.chainB)
				s.coordinator.Setup(path)
				return []interface{}{
					common.BytesToAddress(sender),
					big.NewInt(1e18),
					"random1rhe5leyt5w0mcwd9rpp93zqn99yktsxv8kq5er",
					"aevmos",
					"ibc/3A5B71F2AA11D24F9688A10D4279CE71560489D7A695364FC361EC6E09D02889",
				}
			},
			func(sender, receiver sdk.AccAddress, data []byte, inputArgs []interface{}) {},
			200000,
			true,
			"decoding bech32 failed: invalid checksum",
		},
		{
			"test - check if erc20 contracts work",
			func(sender, receiver sdk.AccAddress) []interface{} {
				path := NewTransferPath(s.chainA, s.chainB)
				s.coordinator.Setup(path)
				return []interface{}{
					s.address,
					big.NewInt(1e18),
					"cosmos1rhe5leyt5w0mcwd9rpp93zqn99yktsxv8kq5er",
					"aevmos",
					"ibc/3A5B71F2AA11D24F9688A10D4279CE71560489D7A695364FC361EC6E09D02889",
				}
			},
			func(sender, receiver sdk.AccAddress, data []byte, inputArgs []interface{}) {},
			200000,
			false,
			"",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()

			sender := s.chainA.SenderAccount.GetAddress()
			receiver := s.chainB.SenderAccount.GetAddress()

			contract := vm.NewContract(vm.AccountRef(common.BytesToAddress(sender)), s.precompile, big.NewInt(0), tc.gas)

			s.ctx = s.ctx.WithGasMeter(sdk.NewInfiniteGasMeter())
			initialGas := s.ctx.GasMeter().GasConsumed()
			s.Require().Zero(initialGas)

			args := tc.malleate(sender, receiver)

			bz, err := s.precompile.Swap(s.ctx, common.BytesToAddress(sender), contract, s.stateDB, &method, args)

			if tc.expError {
				s.Require().ErrorContains(err, tc.errContains)
				s.Require().Empty(bz)
				if tc.postCheck != nil {
					tc.postCheck(sender, receiver, bz, args)
				}
			} else {
				s.Require().NoError(err)
				s.Require().Equal(bz, cmn.TrueValue)
				tc.postCheck(sender, receiver, bz, args)
			}
		})
	}
}
