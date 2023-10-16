// Copyright Tharsis Labs Ltd.(Evmos)
// SPDX-License-Identifier:ENCL-1.0(https://github.com/evmos/evmos/blob/main/LICENSE)

package osmosis


var (
	// ErrTokenPairNotFound is raised when a token pair for a certain address
	// is not found and it is required by the executing function.
	ErrTokenPairNotFound = "token pair for address %s not found"
	// ErrInputTokenNotSupported is raised when a the osmosis outpost receive a non supported
	// input token for the swap.
	ErrInputTokenNotSupported = "input not supported, supported tokens: %v"
	// ErrInvalidSlippagePercentage is raised when the slippage percentage is higher than a pre-defined value.
	ErrInvalidSlippagePercentage = "slippage percentage must be a value between 0 and %d"
	// ErrInvalidWindowSeconds is raised when the window seconds is higher than a pre-defined value.
	ErrInvalidWindowSeconds = "window seconds must be a value between 0 and %d"
)