package keeper_test

import (
	"math/big"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	utiltx "github.com/evmos/evmos/v20/testutil/tx"
	"github.com/evmos/evmos/v20/x/erc20/types"
)

func (suite *KeeperTestSuite) TestBurnCoins() {
	var ctx sdk.Context
	sender := sdk.AccAddress(utiltx.GenerateAddress().Bytes())
	expPair := types.NewTokenPair(utiltx.GenerateAddress(), "coin", types.OWNER_MODULE)
	expPair.SetOwnerAddress(sender.String())
	amount := big.NewInt(1000000)
	id := expPair.GetID()

	testcases := []struct {
		name        string
		malleate    func()
		postCheck   func()
		expErr      bool
		errContains string
	}{
		{
			name: "fail - token pair not found",
			malleate: func() {
				params := types.DefaultParams()
				params.EnableErc20 = true
				suite.network.App.Erc20Keeper.SetParams(ctx, params) //nolint:errcheck
			},
			postCheck:   func() {},
			expErr:      true,
			errContains: "",
		},
		{
			"fail - pair is not native coin",
			func() {
				expPair.ContractOwner = types.OWNER_EXTERNAL
				suite.network.App.Erc20Keeper.SetTokenPair(ctx, expPair)
				suite.network.App.Erc20Keeper.SetDenomMap(ctx, expPair.Denom, id)
				suite.network.App.Erc20Keeper.SetERC20Map(ctx, expPair.GetERC20Contract(), id)
			},
			func() {},
			true,
			types.ErrNonNativeCoinBurningDisabled.Error(),
		},
		{
			"pass",
			func() {
				expPair.ContractOwner = types.OWNER_MODULE
				if err := suite.network.App.BankKeeper.MintCoins(ctx, types.ModuleName, sdk.Coins{{Denom: expPair.Denom, Amount: math.NewIntFromBigInt(amount)}}); err != nil {
					suite.FailNow(err.Error())
				}
				if err := suite.network.App.BankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, sender, sdk.Coins{{Denom: expPair.Denom, Amount: math.NewIntFromBigInt(amount)}}); err != nil {
					suite.FailNow(err.Error())
				}
				expPair.SetOwnerAddress(sender.String())
				suite.network.App.Erc20Keeper.SetTokenPair(ctx, expPair)
				suite.network.App.Erc20Keeper.SetDenomMap(ctx, expPair.Denom, id)
				suite.network.App.Erc20Keeper.SetERC20Map(ctx, expPair.GetERC20Contract(), id)
			},
			func() {
				balance := suite.network.App.BankKeeper.GetBalance(ctx, sender, expPair.Denom)
				suite.Require().Equal(balance.Amount.Int64(), math.NewInt(0).Int64())
			},
			false,
			"",
		},
	}

	for _, tc := range testcases {
		suite.Run(tc.name, func() {
			suite.SetupTest()

			ctx = suite.network.GetContext()

			tc.malleate()

			err := suite.network.App.Erc20Keeper.BurnCoins(ctx, sender, math.NewIntFromBigInt(amount), expPair.Erc20Address)
			if tc.expErr {
				suite.Require().Error(err, "expected transfer transaction to fail")
				suite.Require().Contains(err.Error(), tc.errContains, "expected transfer transaction to fail with specific error")
			} else {
				suite.Require().NoError(err, "expected transfer transaction succeeded")
				tc.postCheck()
			}
		})
	}
}
