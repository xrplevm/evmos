package ante_test

import (
	"fmt"
	"math"
	"math/big"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	client "github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdktestutil "github.com/cosmos/cosmos-sdk/testutil/testdata"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	"github.com/cosmos/cosmos-sdk/x/auth/migrations/legacytx"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	"github.com/cosmos/cosmos-sdk/x/authz"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	"github.com/cosmos/cosmos-sdk/x/staking"
	"github.com/cosmos/cosmos-sdk/x/staking/teststaking"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	cryptocodec "github.com/evmos/ethermint/crypto/codec"
	"github.com/evmos/ethermint/crypto/ethsecp256k1"
	"github.com/evmos/ethermint/encoding"
	"github.com/evmos/ethermint/ethereum/eip712"
	"github.com/evmos/ethermint/tests"
	"github.com/evmos/ethermint/types"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
	feemarkettypes "github.com/evmos/ethermint/x/feemarket/types"
	"github.com/evmos/evmos/v11/app"
	"github.com/evmos/evmos/v11/testutil"
	claimstypes "github.com/evmos/evmos/v11/x/claims/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/tmhash"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmversion "github.com/tendermint/tendermint/proto/tendermint/version"
	"github.com/tendermint/tendermint/version"
)

var (
	s *AnteTestSuite
	_ sdk.AnteHandler = (&MockAnteHandler{}).AnteHandle
)

type AnteTestSuite struct {
	suite.Suite

	ctx       sdk.Context
	app       *app.Evmos
	denom     string
	clientCtx client.Context
}

type MockAnteHandler struct {
	WasCalled bool
	CalledCtx sdk.Context
}

type deductFeeTestCase struct {
	name     string
	balance  sdkmath.Int
	rewards  sdkmath.Int
	gas      uint64
	gasPrice *sdkmath.Int
	checkTx  bool
	simulate bool
}

func (mah *MockAnteHandler) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
	mah.WasCalled = true
	mah.CalledCtx = ctx
	return ctx, nil
}

func (suite *AnteTestSuite) SetupTest(isCheckTx bool) {
	t := suite.T()
	privCons, err := ethsecp256k1.GenerateKey()
	require.NoError(t, err)
	consAddress := sdk.ConsAddress(privCons.PubKey().Address())

	suite.app = app.Setup(isCheckTx, feemarkettypes.DefaultGenesisState())
	suite.ctx = suite.app.BaseApp.NewContext(isCheckTx, tmproto.Header{
		Height:          1,
		ChainID:         "evmos_9001-1",
		Time:            time.Now().UTC(),
		ProposerAddress: consAddress.Bytes(),

		Version: tmversion.Consensus{
			Block: version.BlockProtocol,
		},
		LastBlockId: tmproto.BlockID{
			Hash: tmhash.Sum([]byte("block_id")),
			PartSetHeader: tmproto.PartSetHeader{
				Total: 11,
				Hash:  tmhash.Sum([]byte("partset_header")),
			},
		},
		AppHash:            tmhash.Sum([]byte("app")),
		DataHash:           tmhash.Sum([]byte("data")),
		EvidenceHash:       tmhash.Sum([]byte("evidence")),
		ValidatorsHash:     tmhash.Sum([]byte("validators")),
		NextValidatorsHash: tmhash.Sum([]byte("next_validators")),
		ConsensusHash:      tmhash.Sum([]byte("consensus")),
		LastResultsHash:    tmhash.Sum([]byte("last_result")),
	})

	suite.denom = claimstypes.DefaultClaimsDenom
	evmParams := suite.app.EvmKeeper.GetParams(suite.ctx)
	evmParams.EvmDenom = suite.denom
	suite.app.EvmKeeper.SetParams(suite.ctx, evmParams)

	encodingConfig := encoding.MakeConfig(app.ModuleBasics)
	// We're using TestMsg amino encoding in some tests, so register it here.
	encodingConfig.Amino.RegisterConcrete(&sdktestutil.TestMsg{}, "testdata.TestMsg", nil)
	eip712.SetEncodingConfig(encodingConfig)

	suite.clientCtx = client.Context{}.WithTxConfig(encodingConfig.TxConfig)
}

func TestAnteTestSuite(t *testing.T) {
	s = new(AnteTestSuite)
	suite.Run(t, s)
}

// Commit commits and starts a new block with an updated context.
func (suite *AnteTestSuite) Commit() {
	suite.CommitAfter(time.Second * 0)
}

// Commit commits a block at a given time.
func (suite *AnteTestSuite) CommitAfter(t time.Duration) {
	header := suite.ctx.BlockHeader()
	suite.app.EndBlock(abci.RequestEndBlock{Height: header.Height})
	_ = suite.app.Commit()

	header.Height++
	header.Time = header.Time.Add(t)
	suite.app.BeginBlock(abci.RequestBeginBlock{
		Header: header,
	})

	// update ctx
	suite.ctx = suite.app.BaseApp.NewContext(false, header)
}

func (s *AnteTestSuite) CreateTestTxBuilder(gasPrice sdkmath.Int, denom string, msgs ...sdk.Msg) client.TxBuilder {
	encodingConfig := encoding.MakeConfig(app.ModuleBasics)
	gasLimit := uint64(1000000)

	txBuilder := encodingConfig.TxConfig.NewTxBuilder()

	txBuilder.SetGasLimit(gasLimit)
	fees := &sdk.Coins{{Denom: denom, Amount: gasPrice.MulRaw(int64(gasLimit))}}
	txBuilder.SetFeeAmount(*fees)
	err := txBuilder.SetMsgs(msgs...)
	s.Require().NoError(err)
	return txBuilder
}

func (s *AnteTestSuite) CreateEthTestTxBuilder(msgEthereumTx *evmtypes.MsgEthereumTx) client.TxBuilder {
	encodingConfig := encoding.MakeConfig(app.ModuleBasics)
	option, err := codectypes.NewAnyWithValue(&evmtypes.ExtensionOptionsEthereumTx{})
	s.Require().NoError(err)

	txBuilder := encodingConfig.TxConfig.NewTxBuilder()
	builder, ok := txBuilder.(authtx.ExtensionOptionsTxBuilder)
	s.Require().True(ok)
	builder.SetExtensionOptions(option)

	err = txBuilder.SetMsgs(msgEthereumTx)
	s.Require().NoError(err)

	txData, err := evmtypes.UnpackTxData(msgEthereumTx.Data)
	s.Require().NoError(err)

	fees := sdk.Coins{{Denom: s.denom, Amount: sdk.NewIntFromBigInt(txData.Fee())}}
	builder.SetFeeAmount(fees)
	builder.SetGasLimit(msgEthereumTx.GetGas())

	return txBuilder
}

func (s *AnteTestSuite) BuildTestEthTx(
	from common.Address,
	to common.Address,
	gasPrice *big.Int,
	gasFeeCap *big.Int,
	gasTipCap *big.Int,
	accesses *ethtypes.AccessList,
) *evmtypes.MsgEthereumTx {
	chainID := s.app.EvmKeeper.ChainID()
	nonce := s.app.EvmKeeper.GetNonce(
		s.ctx,
		common.BytesToAddress(from.Bytes()),
	)
	data := make([]byte, 0)
	gasLimit := uint64(100000)
	msgEthereumTx := evmtypes.NewTx(
		chainID,
		nonce,
		&to,
		nil,
		gasLimit,
		gasPrice,
		gasFeeCap,
		gasTipCap,
		data,
		accesses,
	)
	msgEthereumTx.From = from.String()
	return msgEthereumTx
}

var _ sdk.Tx = &invalidTx{}

type invalidTx struct{}

func (invalidTx) GetMsgs() []sdk.Msg   { return []sdk.Msg{nil} }
func (invalidTx) ValidateBasic() error { return nil }

func newMsgGrant(granter sdk.AccAddress, grantee sdk.AccAddress, a authz.Authorization, expiration *time.Time) *authz.MsgGrant {
	msg, err := authz.NewMsgGrant(granter, grantee, a, expiration)
	if err != nil {
		panic(err)
	}
	return msg
}

func newMsgExec(grantee sdk.AccAddress, msgs []sdk.Msg) *authz.MsgExec {
	msg := authz.NewMsgExec(grantee, msgs)
	return &msg
}

func createNestedMsgExec(a sdk.AccAddress, nestedLvl int, lastLvlMsgs []sdk.Msg) *authz.MsgExec {
	msgs := make([]*authz.MsgExec, nestedLvl)
	for i := range msgs {
		if i == 0 {
			msgs[i] = newMsgExec(a, lastLvlMsgs)
			continue
		}
		msgs[i] = newMsgExec(a, []sdk.Msg{msgs[i-1]})
	}
	return msgs[nestedLvl-1]
}

func generatePrivKeyAddressPairs(accCount int) ([]*ethsecp256k1.PrivKey, []sdk.AccAddress, error) {
	var (
		err           error
		testPrivKeys  = make([]*ethsecp256k1.PrivKey, accCount)
		testAddresses = make([]sdk.AccAddress, accCount)
	)

	for i := range testPrivKeys {
		testPrivKeys[i], err = ethsecp256k1.GenerateKey()
		if err != nil {
			return nil, nil, err
		}
		testAddresses[i] = testPrivKeys[i].PubKey().Address().Bytes()
	}
	return testPrivKeys, testAddresses, nil
}

func createTx(priv *ethsecp256k1.PrivKey, msgs ...sdk.Msg) (sdk.Tx, error) {
	encodingConfig := encoding.MakeConfig(app.ModuleBasics)
	txBuilder := encodingConfig.TxConfig.NewTxBuilder()

	txBuilder.SetGasLimit(1000000)
	if err := txBuilder.SetMsgs(msgs...); err != nil {
		return nil, err
	}

	// First round: we gather all the signer infos. We use the "set empty
	// signature" hack to do that.
	sigV2 := signing.SignatureV2{
		PubKey: priv.PubKey(),
		Data: &signing.SingleSignatureData{
			SignMode:  encodingConfig.TxConfig.SignModeHandler().DefaultMode(),
			Signature: nil,
		},
		Sequence: 0,
	}

	sigsV2 := []signing.SignatureV2{sigV2}

	if err := txBuilder.SetSignatures(sigsV2...); err != nil {
		return nil, err
	}

	signerData := authsigning.SignerData{
		ChainID:       "evmos_9000-1",
		AccountNumber: 0,
		Sequence:      0,
	}
	sigV2, err := tx.SignWithPrivKey(
		encodingConfig.TxConfig.SignModeHandler().DefaultMode(), signerData,
		txBuilder, priv, encodingConfig.TxConfig,
		0,
	)
	if err != nil {
		return nil, err
	}

	sigsV2 = []signing.SignatureV2{sigV2}
	err = txBuilder.SetSignatures(sigsV2...)
	if err != nil {
		return nil, err
	}

	return txBuilder.GetTx(), nil
}

func createEIP712CosmosTx(
	from sdk.AccAddress, priv cryptotypes.PrivKey, msgs []sdk.Msg,
) (sdk.Tx, error) {
	var err error

	encodingConfig := encoding.MakeConfig(app.ModuleBasics)
	txBuilder := encodingConfig.TxConfig.NewTxBuilder()

	// GenerateTypedData TypedData
	registry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(registry)
	ethermintCodec := codec.NewProtoCodec(registry)
	cryptocodec.RegisterInterfaces(registry)

	coinAmount := sdk.NewCoin(evmtypes.DefaultEVMDenom, sdk.NewInt(20))
	amount := sdk.NewCoins(coinAmount)
	gas := uint64(200000)

	fee := legacytx.NewStdFee(gas, amount)
	data := legacytx.StdSignBytes("evmos_9000-1", 0, 0, 0, fee, msgs, "", nil)
	typedData, err := eip712.WrapTxToTypedData(ethermintCodec, 9000, msgs[0], data, &eip712.FeeDelegationOptions{
		FeePayer: from,
	})
	if err != nil {
		return nil, err
	}

	sigHash, _, err := apitypes.TypedDataAndHash(typedData)
	if err != nil {
		return nil, err
	}

	// Sign typedData
	keyringSigner := tests.NewSigner(priv)
	signature, pubKey, err := keyringSigner.SignByAddress(from, sigHash)
	if err != nil {
		return nil, err
	}
	signature[crypto.RecoveryIDOffset] += 27 // Transform V from 0/1 to 27/28 according to the yellow paper

	// Add ExtensionOptionsWeb3Tx extension
	var option *codectypes.Any
	option, err = codectypes.NewAnyWithValue(&types.ExtensionOptionsWeb3Tx{
		FeePayer:         from.String(),
		TypedDataChainID: 9000,
		FeePayerSig:      signature,
	})
	if err != nil {
		return nil, err
	}

	builder, _ := txBuilder.(authtx.ExtensionOptionsTxBuilder)

	builder.SetExtensionOptions(option)
	builder.SetFeeAmount(amount)
	builder.SetGasLimit(gas)

	sigsV2 := signing.SignatureV2{
		PubKey: pubKey,
		Data: &signing.SingleSignatureData{
			SignMode: signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON,
		},
		Sequence: 0,
	}

	if err = builder.SetSignatures(sigsV2); err != nil {
		return nil, err
	}

	if err = builder.SetMsgs(msgs...); err != nil {
		return nil, err
	}

	return builder.GetTx(), err
}

// setupDeductFeeTestCase instantiates a new DeductFeeDecorator
// and prepares the accounts with corresponding balance and staking rewards
// Returns the decorator and the tx arguments to use on the test case
func (suite *AnteTestSuite) setupDeductFeeTestCase(addr sdk.AccAddress, priv cryptotypes.PrivKey, tc deductFeeTestCase) (ante.DeductFeeDecorator, authsigning.Tx) {
	suite.SetupTest(tc.checkTx)

	// Create a new DeductFeeDecorator
	dfd := ante.NewDeductFeeDecorator(
		suite.app.AccountKeeper, suite.app.BankKeeper, suite.app.FeeGrantKeeper, nil,
	)

	// prepare the testcase
	err := suite.prepareAccountsForDelegationRewards(addr, tc.balance, tc.rewards)
	suite.Require().NoError(err, "failed to prepare accounts for delegation rewards")
	suite.Commit()

	// Create an arbitrary message for testing purposes
	msg := sdktestutil.NewTestMsg(addr)

	// Set up the transaction arguments
	args := CosmosTxArgs{
		TxCfg:    suite.clientCtx.TxConfig,
		Priv:     priv,
		Gas:      tc.gas,
		GasPrice: tc.gasPrice,
		Msgs:     []sdk.Msg{msg},
	}
	suite.ctx = suite.ctx.WithIsCheckTx(tc.checkTx)

	// Create a transaction out of the message
	tx, err := prepareCosmosTx(suite.ctx, suite.app, args)
	suite.Require().NoError(err)
	return dfd, tx
}

func (suite *AnteTestSuite) prepareAccountsForDelegationRewards(addr sdk.AccAddress, balance, rewards sdkmath.Int) error {
	totalNeededBalance := balance.Add(rewards)
	if totalNeededBalance.IsZero() {
		suite.app.AccountKeeper.SetAccount(suite.ctx, suite.app.AccountKeeper.NewAccountWithAddress(suite.ctx, addr))
	} else {
		// Fund account with enough tokens to stake them
		err := fundAccountWithBaseDenom(suite.ctx, suite.app.BankKeeper, addr, totalNeededBalance.Int64())
		if err != nil {
			return fmt.Errorf("failed to fund account: %s", err.Error())
		}
	}

	if rewards.IsZero() {
		return nil
	}
	// reset historical count in distribution keeper which is necessary
	// for the delegation rewards to be calculated correctly
	suite.app.DistrKeeper.DeleteAllValidatorHistoricalRewards(suite.ctx)

	// set distribution module account balance which pays out the rewards
	distrAcc := suite.app.DistrKeeper.GetDistributionAccount(suite.ctx)
	err := testutil.FundModuleAccount(suite.ctx, suite.app.BankKeeper, distrAcc.GetName(), sdk.NewCoins(sdk.NewCoin(suite.denom, rewards)))
	if err != nil {
		return fmt.Errorf("failed to fund distribution module account: %s", err.Error())
	}
	suite.app.AccountKeeper.SetModuleAccount(suite.ctx, distrAcc)

	// Set up validator and delegate to it
	privKey := ed25519.GenPrivKey()
	addr2, _ := tests.NewAddrKey()
	if err := fundAccountWithBaseDenom(suite.ctx, suite.app.BankKeeper, addr2.Bytes(), rewards.Int64()); err != nil {
		return fmt.Errorf("failed to fund validator account: %s", err.Error())
	}

	zeroDec := sdk.ZeroDec()
	stakingParams := suite.app.StakingKeeper.GetParams(suite.ctx)
	stakingParams.BondDenom = suite.denom
	stakingParams.MinCommissionRate = zeroDec
	suite.app.StakingKeeper.SetParams(suite.ctx, stakingParams)

	stakingHelper := teststaking.NewHelper(suite.T(), suite.ctx, suite.app.StakingKeeper)
	stakingHelper.Commission = stakingtypes.NewCommissionRates(zeroDec, zeroDec, zeroDec)
	stakingHelper.Denom = suite.denom

	valAddr := sdk.ValAddress(addr2.Bytes())
	// self-delegate the same amount of tokens as the delegate address also stakes
	// this ensures, that the delegation rewards are 50% of the total rewards
	stakingHelper.CreateValidator(valAddr, privKey.PubKey(), rewards, true)
	stakingHelper.Delegate(addr, valAddr, rewards)

	// end block to bond validator and increase block height
	// Not using Commit() here because code panics due to invalid block height
	staking.EndBlocker(suite.ctx, suite.app.StakingKeeper)

	// allocate rewards to validator (of these 50% will be paid out to the delegator)
	validator := suite.app.StakingKeeper.Validator(suite.ctx, valAddr)
	allocatedRewards := sdk.NewDecCoins(sdk.NewDecCoin(suite.denom, rewards.Mul(sdk.NewInt(2))))
	suite.app.DistrKeeper.AllocateTokensToValidator(suite.ctx, validator, allocatedRewards)

	return nil
}

// fundAccountWithBaseDenom is a utility function that uses the FundAccount function
// to fund an account with the default Evmos denomination.
func fundAccountWithBaseDenom(ctx sdk.Context, bankKeeper bankkeeper.Keeper, addr sdk.AccAddress, amount int64) error {
	coins := sdk.NewCoins(
		sdk.NewCoin("aevmos", sdk.NewInt(amount)),
	)
	return testutil.FundAccount(ctx, bankKeeper, addr, coins)
}

// CosmosTxArgs contains the params to create a cosmos tx
type CosmosTxArgs struct {
	// TxCfg is the client transaction config
	TxCfg client.TxConfig
	// Priv is the private key that will be used to sign the tx
	Priv cryptotypes.PrivKey
	// ChainID is the chain's id on cosmos format, e.g. 'evmos_9000-1'
	ChainID string
	// Gas to be used on the tx
	Gas uint64
	// GasPrice to use on tx
	GasPrice *sdkmath.Int
	// Fees is the fee to be used on the tx (amount and denom)
	Fees sdk.Coins
	// FeeGranter is the account address of the fee granter
	FeeGranter sdk.AccAddress
	// Msgs slice of messages to include on the tx
	Msgs []sdk.Msg
}

var DefaultFee = sdk.NewCoin("aevmos", sdk.NewIntFromUint64(uint64(math.Pow10(16)))) // 0.01 EVMOS

// prepareCosmosTx creates a cosmos tx and signs it with the provided messages and private key.
// It returns the signed transaction and an error
func prepareCosmosTx(
	ctx sdk.Context,
	appEvmos *app.Evmos,
	args CosmosTxArgs,
) (authsigning.Tx, error) {
	txBuilder := args.TxCfg.NewTxBuilder()

	txBuilder.SetGasLimit(args.Gas)

	var fees sdk.Coins
	if args.GasPrice != nil {
		fees = sdk.Coins{{Denom: "aevmos", Amount: args.GasPrice.MulRaw(int64(args.Gas))}}
	} else {
		fees = sdk.Coins{DefaultFee}
	}

	txBuilder.SetFeeAmount(fees)
	if err := txBuilder.SetMsgs(args.Msgs...); err != nil {
		return nil, err
	}

	txBuilder.SetFeeGranter(args.FeeGranter)

	return signCosmosTx(
		ctx,
		appEvmos,
		args,
		txBuilder,
	)
}

// signCosmosTx signs the cosmos transaction on the txBuilder provided using
// the provided private key
func signCosmosTx(
	ctx sdk.Context,
	appEvmos *app.Evmos,
	args CosmosTxArgs,
	txBuilder client.TxBuilder,
) (authsigning.Tx, error) {
	addr := sdk.AccAddress(args.Priv.PubKey().Address().Bytes())
	seq, err := appEvmos.AccountKeeper.GetSequence(ctx, addr)
	if err != nil {
		return nil, err
	}

	// First round: we gather all the signer infos. We use the "set empty
	// signature" hack to do that.
	sigV2 := signing.SignatureV2{
		PubKey: args.Priv.PubKey(),
		Data: &signing.SingleSignatureData{
			SignMode:  args.TxCfg.SignModeHandler().DefaultMode(),
			Signature: nil,
		},
		Sequence: seq,
	}

	sigsV2 := []signing.SignatureV2{sigV2}

	if err := txBuilder.SetSignatures(sigsV2...); err != nil {
		return nil, err
	}

	// Second round: all signer infos are set, so each signer can sign.
	accNumber := appEvmos.AccountKeeper.GetAccount(ctx, addr).GetAccountNumber()
	signerData := authsigning.SignerData{
		ChainID:       args.ChainID,
		AccountNumber: accNumber,
		Sequence:      seq,
	}
	sigV2, err = tx.SignWithPrivKey(
		args.TxCfg.SignModeHandler().DefaultMode(),
		signerData,
		txBuilder, args.Priv, args.TxCfg,
		seq,
	)
	if err != nil {
		return nil, err
	}

	sigsV2 = []signing.SignatureV2{sigV2}
	if err = txBuilder.SetSignatures(sigsV2...); err != nil {
		return nil, err
	}
	return txBuilder.GetTx(), nil
}
