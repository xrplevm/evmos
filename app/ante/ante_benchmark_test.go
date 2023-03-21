package ante_test

import (
	"fmt"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/evmos/ethermint/tests"
)

// This tests setup contains expensive operations.
// Make sure to run this benchmark tests with a limited number of iterations
// To do so, specify the iteration num with the -benchtime flag
// e.g.: go test -bench=DeductFeeDecorator -benchtime=1000x
func BenchmarkDeductFeeDecorator(b *testing.B) {
	s := new(AnteTestSuite)
	s.SetT(&testing.T{})
	s.SetupTest(false)
	mmd := MockAnteHandler{}

	testCases := []deductFeeTestCase{
		{
			name:     "sufficient balance to pay fees",
			balance:  sdk.NewInt(1e18),
			rewards:  sdk.ZeroInt(),
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
