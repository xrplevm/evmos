package keeper

import (
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/evmos/evmos/v20/x/erc20/types"
)

// TransferOwnershipProposal transfers ownership of the token to the new owner through a proposal
func (k Keeper) TransferOwnershipProposal(ctx sdk.Context, newOwner sdk.AccAddress, token string) error {
	pair, found := k.GetTokenPair(ctx, k.GetTokenPairID(ctx, token))
	if !found {
		return errorsmod.Wrapf(types.ErrTokenPairNotFound, "token '%s' not registered", token)
	}

	return k.transferOwnership(ctx, newOwner, pair)
}

// TransferOwnership transfers ownership of the token to the new owner.
func (k Keeper) TransferOwnership(ctx sdk.Context, sender, newOwner sdk.AccAddress, token string) error {
	pair, found := k.GetTokenPair(ctx, k.GetTokenPairID(ctx, token))
	if !found {
		return errorsmod.Wrapf(types.ErrTokenPairNotFound, "token '%s' not registered", token)
	}

	ownerAddr, err := sdk.AccAddressFromBech32(pair.OwnerAddress)
	if err != nil {
		return errorsmod.Wrapf(err, "invalid owner address")
	}

	if !sender.Equals(ownerAddr) {
		return errorsmod.Wrap(types.ErrMinterIsNotOwner, "sender is not the owner of the token")
	}

	return k.transferOwnership(ctx, newOwner, pair)
}

// transferOwnership transfers ownership of the token to the new owner
func (k Keeper) transferOwnership(ctx sdk.Context, newOwner sdk.AccAddress, token types.TokenPair) error {
	if !token.IsNativeCoin() {
		return errorsmod.Wrap(types.ErrNonNativeTransferOwnershipDisabled, token.Erc20Address)
	}

	k.SetTokenPairOwnerAddress(ctx, token, newOwner.String())

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyAction, types.TypeMsgTransferOwnership),
			sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
			sdk.NewAttribute(types.AttributeKeyNewOwner, newOwner.String()),
		),
	)

	return nil
}

func (k Keeper) GetOwnerAddress(ctx sdk.Context, contractAddress string) string {
	pair, found := k.GetTokenPair(ctx, k.GetTokenPairID(ctx, contractAddress))
	if !found {
		return ""
	}

	return pair.OwnerAddress
}
