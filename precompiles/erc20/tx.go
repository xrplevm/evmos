// Copyright Tharsis Labs Ltd.(Evmos)
// SPDX-License-Identifier:ENCL-1.0(https://github.com/evmos/evmos/blob/main/LICENSE)
package erc20

import (
	"math/big"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	cmn "github.com/evmos/evmos/v19/precompiles/common"
	"github.com/evmos/evmos/v19/utils"
	erc20types "github.com/evmos/evmos/v19/x/erc20/types"
	"github.com/evmos/evmos/v19/x/evm/core/vm"
)

const (
	// TransferMethod defines the ABI method name for the ERC-20 transfer
	// transaction.
	TransferMethod = "transfer"
	// TransferFromMethod defines the ABI method name for the ERC-20 transferFrom
	// transaction.
	TransferFromMethod = "transferFrom"
	// MintMethod defines the ABI method name for the ERC-20 mint transaction.
	MintMethod = "mint"
	// BurnMethod defines the ABI method name for the ERC-20 burn transaction.
	BurnMethod = "burn"
)

// SendMsgURL defines the authorization type for MsgSend
var SendMsgURL = sdk.MsgTypeURL(&banktypes.MsgSend{})

// Transfer executes a direct transfer from the caller address to the
// destination address.
func (p *Precompile) Transfer(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	from := contract.CallerAddress
	to, amount, err := ParseTransferArgs(args)
	if err != nil {
		return nil, err
	}

	return p.transfer(ctx, contract, stateDB, method, from, to, amount)
}

// TransferFrom executes a transfer on behalf of the specified from address in
// the call data to the destination address.
func (p *Precompile) TransferFrom(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	from, to, amount, err := ParseTransferFromArgs(args)
	if err != nil {
		return nil, err
	}

	return p.transfer(ctx, contract, stateDB, method, from, to, amount)
}

// transfer is a common function that handles transfers for the ERC-20 Transfer
// and TransferFrom methods. It executes a bank Send message if the spender is
// the sender of the transfer, otherwise it executes an authorization.
func (p *Precompile) transfer(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	from, to common.Address,
	amount *big.Int,
) (data []byte, err error) {
	coins := sdk.Coins{{Denom: p.tokenPair.Denom, Amount: math.NewIntFromBigInt(amount)}}

	msg := banktypes.NewMsgSend(from.Bytes(), to.Bytes(), coins)

	if err = msg.ValidateBasic(); err != nil {
		return nil, err
	}

	isTransferFrom := method.Name == TransferFromMethod
	owner := sdk.AccAddress(from.Bytes())
	spenderAddr := contract.CallerAddress
	spender := sdk.AccAddress(spenderAddr.Bytes()) // aka. grantee
	ownerIsSpender := spender.Equals(owner)

	var prevAllowance *big.Int
	if ownerIsSpender {
		msgSrv := bankkeeper.NewMsgServerImpl(p.bankKeeper)
		_, err = msgSrv.Send(sdk.WrapSDKContext(ctx), msg)
	} else {
		_, _, prevAllowance, err = GetAuthzExpirationAndAllowance(p.AuthzKeeper, ctx, spenderAddr, from, p.tokenPair.Denom)
		if err != nil {
			return nil, ConvertErrToERC20Error(errorsmod.Wrapf(authz.ErrNoAuthorizationFound, err.Error()))
		}

		_, err = p.AuthzKeeper.DispatchActions(ctx, spender, []sdk.Msg{msg})
	}
	if err != nil {
		err = ConvertErrToERC20Error(err)
		// This should return an error to avoid the contract from being executed and an event being emitted
		return nil, err
	}

	// TODO: where should we get this
	if p.tokenPair.Denom == utils.BaseDenom {
		p.SetBalanceChangeEntries(cmn.NewBalanceChangeEntry(from, msg.Amount.AmountOf(utils.BaseDenom).BigInt(), cmn.Sub),
			cmn.NewBalanceChangeEntry(to, msg.Amount.AmountOf(utils.BaseDenom).BigInt(), cmn.Add))
	}

	if err = p.EmitTransferEvent(ctx, stateDB, from, to, amount); err != nil {
		return nil, err
	}

	// NOTE: if it's a direct transfer, we return here but if used through transferFrom,
	// we need to emit the approval event with the new allowance.
	if !isTransferFrom {
		return method.Outputs.Pack(true)
	}

	var newAllowance *big.Int
	if ownerIsSpender {
		// NOTE: in case the spender is the owner we emit an approval event with
		// the maxUint256 value.
		newAllowance = abi.MaxUint256
	} else {
		newAllowance = new(big.Int).Sub(prevAllowance, amount)
	}

	if err = p.EmitApprovalEvent(ctx, stateDB, from, spenderAddr, newAllowance); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// Mint executes a mint of the caller's tokens.
func (p *Precompile) Mint(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	to, amount, err := ParseMintArgs(args)
	if err != nil {
		return nil, err
	}

	// TODO: Check minter is the owner of the token
	minterAddr := contract.CallerAddress
	_ = sdk.AccAddress(minterAddr.Bytes())
	// minterIsOwner := minter.Equals(sdk.AccAddress(to.Bytes()))
	minterIsOwner := true

	if !minterIsOwner {
		return nil, ConvertErrToERC20Error(errorsmod.Wrapf(authz.ErrNoAuthorizationFound, "minter is not the owner"))
	}

	coins := sdk.Coins{{Denom: p.tokenPair.Denom, Amount: math.NewIntFromBigInt(amount)}}
	err = p.bankKeeper.MintCoins(ctx, erc20types.ModuleName, coins)
	if err != nil {
		return nil, err
	}

	err = p.bankKeeper.SendCoinsFromModuleToAccount(ctx, erc20types.ModuleName, sdk.AccAddress(to.Bytes()), coins)
	if err != nil {
		return nil, err
	}

	if p.tokenPair.Denom == utils.BaseDenom {
		p.SetBalanceChangeEntries(
			cmn.NewBalanceChangeEntry(to, coins.AmountOf(utils.BaseDenom).BigInt(), cmn.Add))
	}

	moduleAccount := p.accountKeeper.GetModuleAccount(ctx, erc20types.ModuleName)

	if err = p.EmitTransferEvent(ctx, stateDB, to, common.Address(moduleAccount.GetAddress().Bytes()), amount); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// Burn executes a burn of the caller's tokens.
func (p *Precompile) Burn(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	amount, err := ParseBurnArgs(args)
	if err != nil {
		return nil, err
	}

	burnerAddr := contract.CallerAddress
	burner := sdk.AccAddress(burnerAddr.Bytes())
	// TODO: Replace with Ownable
	// burnerIsOwner := burner.Equals(sdk.AccAddress(from.Bytes()))
	burnerIsOwner := true

	if !burnerIsOwner {
		return nil, ConvertErrToERC20Error(errorsmod.Wrapf(authz.ErrNoAuthorizationFound, "burner is not the owner"))
	}
	coins := sdk.Coins{{Denom: p.tokenPair.Denom, Amount: math.NewIntFromBigInt(amount)}}

	err = p.bankKeeper.SendCoinsFromAccountToModule(ctx, burner, erc20types.ModuleName, coins)
	if err != nil {
		return nil, err
	}

	moduleAccount := p.accountKeeper.GetModuleAccount(ctx, erc20types.ModuleName)

	err = p.bankKeeper.BurnCoins(ctx, erc20types.ModuleName, coins)
	if err != nil {
		return nil, err
	}

	if p.tokenPair.Denom == utils.BaseDenom {
		p.SetBalanceChangeEntries(
			cmn.NewBalanceChangeEntry(burnerAddr, coins.AmountOf(utils.BaseDenom).BigInt(), cmn.Sub))
	}

	if err = p.EmitTransferEvent(ctx, stateDB, burnerAddr, common.Address(moduleAccount.GetAddress().Bytes()), amount); err != nil {
		return nil, err
	}

	return method.Outputs.Pack()
}