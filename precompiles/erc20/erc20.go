// Copyright Tharsis Labs Ltd.(Evmos)
// SPDX-License-Identifier:ENCL-1.0(https://github.com/evmos/evmos/blob/main/LICENSE)

package erc20

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	cmn "github.com/evmos/evmos/v15/precompiles/common"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	auth "github.com/evmos/evmos/v15/precompiles/authorization"
	erc20types "github.com/evmos/evmos/v15/x/erc20/types"
)

// abiPath defines the path to the staking precompile ABI JSON file.
const abiPath = "./abi.json"

var _ vm.PrecompiledContract = &Precompile{}

// Precompile defines the precompiled contract for staking.
type Precompile struct {
	cmn.Precompile
	abi.ABI
	tokenPair          erc20types.TokenPair
	bankKeeper         bankkeeper.Keeper
	authzKeeper        authzkeeper.Keeper
	approvalExpiration time.Duration
}

// NewPrecompile creates a new staking Precompile instance as a
// PrecompiledContract interface.
func NewPrecompile(
	tokenPair erc20types.TokenPair,
	bankKeeper bankkeeper.Keeper,
	authzKeeper authzkeeper.Keeper,
) (*Precompile, error) {
	abiJSON, err := os.ReadFile(filepath.Clean(abiPath))
	if err != nil {
		return nil, fmt.Errorf("failed to open newAbi.json file: %w", err)
	}

	newAbi, err := abi.JSON(strings.NewReader(string(abiJSON)))
	if err != nil {
		return nil, fmt.Errorf("invalid newAbi.json file: %w", err)
	}

	return &Precompile{
		ABI:                newAbi,
		tokenPair:          tokenPair,
		bankKeeper:         bankKeeper,
		authzKeeper:        authzKeeper,
		approvalExpiration: time.Hour * 24 * 365,
	}, nil
}

// Address defines the address of the ERC20 precompile contract.
func (p Precompile) Address() common.Address {
	return p.tokenPair.GetERC20Contract()
}

// IsStateful returns true since the precompile contract has access to the
// staking state.
func (Precompile) IsStateful() bool {
	return true
}

// RequiredGas calculates the contract gas used for the
func (p Precompile) RequiredGas(input []byte) uint64 {
	// Validate input length
	if len(input) < 4 {
		return 0
	}

	methodID := input[:4]
	method, err := p.MethodById(methodID)
	if err != nil {
		return 0
	}

	// TODO: these values were obtained from Remix using the ERC20.sol from OpenZeppelin.
	// We should execute the transactions using the ERC20MinterBurnerDecimals.sol from Evmos testnet
	// to ensure parity in the values.
	switch method.Name {
	// ERC20 transactions
	case TransferMethod:
		return 3_000_000
	case TransferFromMethod:
		return 3_000_000
	case auth.ApproveMethod:
		return 30_956
	case auth.IncreaseAllowanceMethod:
		return 34_605
	case auth.DecreaseAllowanceMethod:
		return 34_519
	// ERC20 queries
	case NameMethod:
		return 3_421
	case SymbolMethod:
		return 3_464
	case DecimalsMethod:
		return 427
	case TotalSupplyMethod:
		return 2_477
	case BalanceOfMethod:
		return 2_851
	case auth.AllowanceMethod:
		return 3_246
	default:
		return 0
	}
}

// Run executes the precompiled contract staking methods defined in the ABI.
func (p Precompile) Run(evm *vm.EVM, contract *vm.Contract, readOnly bool) (bz []byte, err error) {
	ctx, stateDB, method, initialGas, args, err := p.RunSetup(evm, contract, readOnly, p.IsTransaction)
	if err != nil {
		return nil, err
	}

	// This handles any out of gas errors that may occur during the execution of a precompile tx or query.
	// It avoids panics and returns the out of gas error so the EVM can continue gracefully.
	defer cmn.HandleGasError(ctx, contract, initialGas, &err)()

	bz, err = p.HandleMethod(ctx, contract, stateDB, method, args)
	if err != nil {
		return nil, err
	}

	cost := ctx.GasMeter().GasConsumed() - initialGas

	if !contract.UseGas(cost) {
		return nil, vm.ErrOutOfGas
	}

	return bz, nil
}

// IsTransaction checks if the given methodID corresponds to a transaction or query.
func (Precompile) IsTransaction(methodID string) bool {
	switch methodID {
	case TransferMethod,
		TransferFromMethod,
		auth.ApproveMethod,
		auth.IncreaseAllowanceMethod,
		auth.DecreaseAllowanceMethod:
		return true
	default:
		return false
	}
}

func (p Precompile) HandleMethod(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) (bz []byte, err error) {
	switch method.Name {
	// ERC20 transactions
	case TransferMethod:
		bz, err = p.Transfer(ctx, contract, stateDB, method, args)
	case TransferFromMethod:
		bz, err = p.TransferFrom(ctx, contract, stateDB, method, args)
	case auth.ApproveMethod:
		bz, err = p.Approve(ctx, contract, stateDB, method, args)
	case auth.IncreaseAllowanceMethod:
		bz, err = p.IncreaseAllowance(ctx, contract, stateDB, method, args)
	case auth.DecreaseAllowanceMethod:
		bz, err = p.DecreaseAllowance(ctx, contract, stateDB, method, args)
	// ERC20 queries
	case NameMethod:
		bz, err = p.Name(ctx, contract, stateDB, method, args)
	case SymbolMethod:
		bz, err = p.Symbol(ctx, contract, stateDB, method, args)
	case DecimalsMethod:
		bz, err = p.Decimals(ctx, contract, stateDB, method, args)
	case TotalSupplyMethod:
		bz, err = p.TotalSupply(ctx, contract, stateDB, method, args)
	case BalanceOfMethod:
		bz, err = p.BalanceOf(ctx, contract, stateDB, method, args)
	case auth.AllowanceMethod:
		bz, err = p.Allowance(ctx, contract, stateDB, method, args)
	}

	return bz, err
}