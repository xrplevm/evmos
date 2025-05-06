package keeper

import (
	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/evmos/evmos/v20/x/erc20/types"
)

// BurnCoins burns the provided amount of coins from the given address.
func (k Keeper) BurnCoins(ctx sdk.Context, sender sdk.AccAddress, amount math.Int, token string) error {
	pair, found := k.GetTokenPair(ctx, k.GetTokenPairID(ctx, token))
	if !found {
		return errorsmod.Wrapf(types.ErrTokenPairNotFound, "token '%s' not registered", token)
	}

	if !pair.IsNativeCoin() {
		return errorsmod.Wrap(types.ErrNonNativeCoinBurningDisabled, token)
	}

	coins := sdk.Coins{{Denom: pair.Denom, Amount: amount}}

	err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, sender, types.ModuleName, coins)
	if err != nil {
		return err
	}

	err = k.bankKeeper.BurnCoins(ctx, types.ModuleName, coins)
	if err != nil {
		return err
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyAction, types.TypeMsgBurn),
			sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
			sdk.NewAttribute(sdk.AttributeKeySender, sender.String()),
			sdk.NewAttribute(sdk.AttributeKeyAmount, amount.String()),
		),
	)
	return nil
}
