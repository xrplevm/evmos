package ante_test

import (
	"fmt"
	"math/big"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	ethante "github.com/evmos/ethermint/app/ante"
	"github.com/evmos/ethermint/server/config"
	"github.com/evmos/ethermint/tests"
	"github.com/evmos/ethermint/x/evm/statedb"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
)

var mmd = MockAnteHandler{}

// This tests setup contains expensive operations.
// Make sure to run this benchmark tests with a limited number of iterations
// To do so, specify the iteration num with the -benchtime flag
// e.g.: go test -bench=DeductFeeDecorator -benchtime=1000x
func BenchmarkDeductFeeDecorator(b *testing.B) {
	s := new(AnteTestSuite)
	s.SetT(&testing.T{})
	s.SetupTest(false)

	testCases := []deductFeeTestCase{
		{
			name:     "sufficient balance to pay fees",
			balance:  sdk.NewInt(1e18),
			simulate: true,
		},
	}

	b.ResetTimer()

	for _, tc := range testCases {
		b.Run(fmt.Sprintf("Case: %s", tc.name), func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				// Stop the timer to perform expensive test setup
				b.StopTimer()
				addr, priv := tests.NewAddrKey()

				// Create a new DeductFeeDecorator
				dfd, tx := s.setupDeductFeeTestCase(addr.Bytes(), priv, tc)

				// Benchmark only the ante handler logic - start the timer
				b.StartTimer()
				_, err := dfd.AnteHandle(s.ctx, tx, tc.simulate, mmd.AnteHandle)
				s.Require().NoError(err)
			}
		})
	}
}

func BenchmarkEthGasConsumeDecorator(b *testing.B) {
	s := new(AnteTestSuite)
	s.SetT(&testing.T{})
	s.SetupTest(false)

	dec := ethante.NewEthGasConsumeDecorator(s.app.EvmKeeper, config.DefaultMaxTxGasWanted)

	var vmdb *statedb.StateDB

	testCases := []deductFeeTestCase{
		{
			name:    "legacy tx - enough funds to pay for fees",
			balance: sdk.NewInt(1e16),
		},
	}
	b.ResetTimer()

	for _, tc := range testCases {
		b.Run(fmt.Sprintf("Case %s", tc.name), func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				// Stop the timer to perform expensive test setup
				b.StopTimer()
				addr, _ := tests.NewAddrKey()

				tx := evmtypes.NewTx(
					s.app.EvmKeeper.ChainID(),
					1,
					nil,
					big.NewInt(10),
					uint64(1000000),
					big.NewInt(1000000000),
					nil, nil, nil,
					&ethtypes.AccessList{{Address: addr, StorageKeys: nil}},
				)
				tx.From = addr.Hex()

				cacheCtx, _ := s.ctx.CacheContext()
				// Create new stateDB for each test case from the cached context
				vmdb = statedb.New(s.ctx, s.app.EvmKeeper, statedb.NewEmptyTxConfig(common.BytesToHash(s.ctx.HeaderHash().Bytes())))
				err := fundAccountWithBaseDenom(s.ctx, s.app.BankKeeper, addr.Bytes(), tc.balance.Int64())
				s.Require().NoError(err)
				cacheCtx = cacheCtx.
					WithBlockGasMeter(sdk.NewGasMeter(1e19)).
					WithBlockHeight(cacheCtx.BlockHeight() + 1)
				s.Require().NoError(vmdb.Commit())

				// Benchmark only the ante handler logic - start the timer
				b.StartTimer()
				_, err = dec.AnteHandle(cacheCtx.WithIsCheckTx(true).WithGasMeter(sdk.NewInfiniteGasMeter()), tx, tc.simulate, mmd.AnteHandle)
				s.Require().NoError(err)
			}
		})
	}
}
