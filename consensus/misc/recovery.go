// Copyright 2026 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package misc

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
)

// RecoveryDebit is one account the recovery fork takes native balance from.
type RecoveryDebit struct {
	Address common.Address
	// Amount in wei. If the account holds less than this at the fork block,
	// only its whole balance is taken (the shortfall is logged).
	Amount *uint256.Int
}

// RecoveryStorageWrite sets one storage slot of a contract at the fork block,
// e.g. to repair the internal bookkeeping of a debited contract.
type RecoveryStorageWrite struct {
	Address common.Address
	Slot    common.Hash
	Value   common.Hash
}

// RecoveryForkConfig describes a one-off irregular state change applied at the
// very start of a single block, before any system call or transaction (the same
// place the DAO fork is applied).
//
// It is keyed on chain ID and block number and is deliberately NOT a field of
// params.ChainConfig: fields there ending in "Block"/"Time" are picked up by
// core/forkid via reflection, and a block-number fork placed after the
// time-based forks would change the fork ID immediately and split upgraded and
// non-upgraded peers long before the fork block.
type RecoveryForkConfig struct {
	ChainID     uint64 // chain the fork applies to; 0 disables the fork
	BlockNumber uint64 // block at which the state change is applied; 0 disables the fork

	Recipient     common.Address // operator-controlled wallet receiving the sum of all debits
	Debits        []RecoveryDebit
	ExpectedTotal *uint256.Int // must equal the sum of Debits[i].Amount (typo guard)

	StorageWrites []RecoveryStorageWrite // optional, applied after the balance moves
}

var (
	fameGenesisPool = common.HexToAddress("0x2e2a3e2671a749a384efd7894fbc9b357140aaae")
	fameToken       = common.HexToAddress("0x0466e0f1ee4cf3ca2873af7c3690502150516731")
)

// EnduranceRecovery is the recovery fork enforced by this client.
//
// The 35 debit caps and their aggregate are frozen at Endurance block 7,155,515.
// The recovery ledger records the source hashes and independently verifiable balances.
var EnduranceRecovery = &RecoveryForkConfig{
	ChainID:     648,
	BlockNumber: 7_155_516,
	Recipient:   common.HexToAddress("0xE855eE9F32E7C97EF7a4255d8bc0d4c439cB6740"),
	Debits: []RecoveryDebit{
		{Address: common.HexToAddress("0xdad12ece4e55fb4f82f8ea7e4935c0aea4cef7d4"), Amount: uint256.MustFromDecimal("45877855737944272799706000")},
		{Address: common.HexToAddress("0xfd793499a8641be0f7ac19c558d53f00c3bfeb03"), Amount: uint256.MustFromDecimal("5856099999992839000000000")},
		{Address: fameGenesisPool, Amount: uint256.MustFromDecimal("2000000000000000000000000")},
		{Address: common.HexToAddress("0xc50c1fc0eed8f740a9efba01efa4fac1f5d3b11c"), Amount: uint256.MustFromDecimal("999999994959999664000")},
		{Address: common.HexToAddress("0x8d18703ac2736193fbdd03e33d696b7d6df1eae7"), Amount: uint256.MustFromDecimal("212935749943531154000")},
		{Address: common.HexToAddress("0x8e0cd5b5323c203096fdac1581535c0e5662a59b"), Amount: uint256.MustFromDecimal("120000000231000000000")},
		{Address: common.HexToAddress("0xe09cf8f1cc9416869604b588a2c33fb62b90f67e"), Amount: uint256.MustFromDecimal("101999994799999706000")},
		{Address: common.HexToAddress("0x0953bf32ac538e5037e8bdc683b78b0aa265022b"), Amount: uint256.MustFromDecimal("99999995799999706000")},
		{Address: common.HexToAddress("0x8851a6924db7ed6c60ef86377c4ecfe8db4dc89b"), Amount: uint256.MustFromDecimal("99999992414799412000")},
		{Address: common.HexToAddress("0x1558ad8ed679d00f7c419291b4b32da1a59b06e8"), Amount: uint256.MustFromDecimal("91999941515195906064")},
		{Address: common.HexToAddress("0xeb2e10814e24d82876dd143f4af2aefcea129f9a"), Amount: uint256.MustFromDecimal("90384297559868277029")},
		{Address: common.HexToAddress("0x9a8691082599f7cd11de009aac2e326b35622d7e"), Amount: uint256.MustFromDecimal("80999997878999853000")},
		{Address: common.HexToAddress("0x19d958a00ebeac90123615967f1e71e2fe846f84"), Amount: uint256.MustFromDecimal("40700000000000000000")},
		{Address: common.HexToAddress("0x722d88c792b1649ce2ca021fb4235580ff8d050c"), Amount: uint256.MustFromDecimal("12000001000000000000")},
		{Address: common.HexToAddress("0x9a7dd2aafcd51f0314d441d5c514cf1726a056b9"), Amount: uint256.MustFromDecimal("6020485977459563784")},
		{Address: common.HexToAddress("0x3f746dc7103f6da61be5572da25c9d728f5eb080"), Amount: uint256.MustFromDecimal("1999991599999412000")},
		{Address: common.HexToAddress("0xf9b300237ad8e1ab26ce71c9b8d0fb9ad2c70c01"), Amount: uint256.MustFromDecimal("1999989499999265000")},
		{Address: common.HexToAddress("0xff013a8dd32183a5149d54e173ab8ff5240b8621"), Amount: uint256.MustFromDecimal("999997899999853000")},
		{Address: common.HexToAddress("0x5759ab47e8a3ec4b6189ed674391c2a2b6c83fc3"), Amount: uint256.MustFromDecimal("999995757999706000")},
		{Address: common.HexToAddress("0xb5e9191514e53ef6cc544878f81357a31c8d92e2"), Amount: uint256.MustFromDecimal("401814934799559000")},
		{Address: common.HexToAddress("0x1e3962b4021665b2ded6d46ebab42d8643401807"), Amount: uint256.MustFromDecimal("1110999925906994813489")},
		{Address: common.HexToAddress("0xdc7a3d4b40a5ef61860d0ea1849ffb639731eb86"), Amount: uint256.MustFromDecimal("2100000147000")},
		{Address: common.HexToAddress("0x442d7e897e0acfb6356313a80d274b9aadf89bf8"), Amount: uint256.MustFromDecimal("231099853000")},
		{Address: common.HexToAddress("0x6e737bf3dc18a9d772ac5f3f94fae8675f951904"), Amount: uint256.MustFromDecimal("231099706000")},
		{Address: common.HexToAddress("0xa98050f1c24a5f2f8cb94657c3852cbd973bfc03"), Amount: uint256.MustFromDecimal("231054736086")},
		{Address: common.HexToAddress("0xe1fb29bdb482e29750f030ae89ac3e09472445d8"), Amount: uint256.MustFromDecimal("231017320400")},
		{Address: common.HexToAddress("0xb0081f141ad8f7e2eff25666caf75488ed4a45fe"), Amount: uint256.MustFromDecimal("231000000000")},
		{Address: common.HexToAddress("0x438f558256b2c0f787d4fc2657b45bce9d8587ed"), Amount: uint256.MustFromDecimal("231000000000")},
		{Address: common.HexToAddress("0x58e279be5c713f8e0af097daa22e1ceca75fa725"), Amount: uint256.MustFromDecimal("231000000000")},
		{Address: common.HexToAddress("0x9b391c4c7a0b44c5adcfaffcd9783c5bf7fcebfd"), Amount: uint256.MustFromDecimal("231000000000")},
		{Address: common.HexToAddress("0x5e922314babadf7058265934c275ca2adbcd4751"), Amount: uint256.MustFromDecimal("231000000000")},
		{Address: common.HexToAddress("0x663365cf9c6cb6830fa3c52bec38f8ba3fb5fee8"), Amount: uint256.MustFromDecimal("231000000000")},
		{Address: common.HexToAddress("0x7bd0667dd2d254d53428706460771219384e35d7"), Amount: uint256.MustFromDecimal("231000000000")},
		{Address: common.HexToAddress("0xe05744082f50850fe75177b2a9552f73cb4b8280"), Amount: uint256.MustFromDecimal("231000000000")},
		{Address: common.HexToAddress("0x87e8bb6681a481693e1b56b2d5d7de44a199edf9"), Amount: uint256.MustFromDecimal("231000000000")},
	},
	ExpectedTotal: uint256.MustFromDecimal("53737030180109895717318852"),
	// Post-values computed from Endurance block 7,155,515 and applied before any fork-block transactions.
	StorageWrites: []RecoveryStorageWrite{
		{ // Clear the two deposits recorded together as season 12 principal.
			Address: fameGenesisPool,
			Slot:    common.HexToHash("0x4d4cbf49c057812f47683eb55fdda39ada1421fad3104dc125d0268bdf0994b4"),
			Value:   common.Hash{},
		},
		{ // _balances[0x1558ad8ed679d00f7c419291b4b32da1a59b06e8] after burning 2,000,000 FAME.
			Address: fameToken,
			Slot:    common.HexToHash("0xb52f4458f7681650438b38548fc3d708e71143c303ea948613f8311975f71c6a"),
			Value:   uint256.MustFromDecimal("3191111535819033305").Bytes32(),
		},
		{ // _totalSupply after the same burn, in 18-decimal base units.
			Address: fameToken,
			Slot:    common.HexToHash("0x02"),
			Value:   uint256.MustFromDecimal("27256410000000000000000").Bytes32(),
		},
	},
}

// activeRecovery is the configuration consulted by the block processing code.
var activeRecovery = EnduranceRecovery

func init() {
	if err := EnduranceRecovery.Validate(); err != nil {
		panic(fmt.Sprintf("invalid recovery fork configuration: %v", err))
	}
}

// Enabled reports whether the configuration activates a fork at all.
func (r *RecoveryForkConfig) Enabled() bool {
	return r != nil && r.ChainID != 0 && r.BlockNumber != 0
}

// Validate checks the configuration for mistakes that would otherwise only show
// up at the fork block.
func (r *RecoveryForkConfig) Validate() error {
	if !r.Enabled() {
		return nil
	}
	if r.Recipient == (common.Address{}) {
		return errors.New("recipient address not set")
	}
	if len(r.Debits) == 0 {
		return errors.New("no debits configured")
	}
	if r.ExpectedTotal == nil {
		return errors.New("expected total not set")
	}
	var (
		seen  = make(map[common.Address]bool, len(r.Debits))
		total = new(uint256.Int)
	)
	for i, d := range r.Debits {
		if d.Address == (common.Address{}) {
			return fmt.Errorf("debit %d: zero address", i)
		}
		if d.Address == r.Recipient {
			return fmt.Errorf("debit %d: recipient address %s cannot be debited", i, d.Address)
		}
		if seen[d.Address] {
			return fmt.Errorf("debit %d: duplicate address %s", i, d.Address)
		}
		seen[d.Address] = true
		if d.Amount == nil || d.Amount.IsZero() {
			return fmt.Errorf("debit %d (%s): amount must be positive", i, d.Address)
		}
		if _, overflow := total.AddOverflow(total, d.Amount); overflow {
			return fmt.Errorf("debit %d (%s): total overflows", i, d.Address)
		}
	}
	if !total.Eq(r.ExpectedTotal) {
		return fmt.Errorf("sum of debits %s does not match expected total %s", total.Dec(), r.ExpectedTotal.Dec())
	}
	return nil
}

// IsRecoveryBlock reports whether the recovery state change must be applied at
// the start of the given block on the given chain.
func IsRecoveryBlock(config *params.ChainConfig, number uint64) bool {
	r := activeRecovery
	if !r.Enabled() || number != r.BlockNumber {
		return false
	}
	return config.ChainID != nil && config.ChainID.IsUint64() && config.ChainID.Uint64() == r.ChainID
}

// ApplyRecoveryHardFork moves the configured balances into the recipient wallet
// and applies the configured storage writes. It must be called with the parent
// block's state, before any system call or transaction of the fork block, by
// every code path that executes or builds that block.
//
// The result only depends on the state it is given, so every node computes the
// same post-state: an account holding less than its configured amount is
// drained to zero and the recipient receives what was actually taken.
func ApplyRecoveryHardFork(statedb vm.StateDB) {
	r := activeRecovery
	taken := new(uint256.Int)
	for _, d := range r.Debits {
		amount := new(uint256.Int).Set(d.Amount)
		if balance := statedb.GetBalance(d.Address); balance.Lt(amount) {
			log.Warn("Recovery fork debit short", "account", d.Address, "want", d.Amount.Dec(), "have", balance.Dec())
			amount.Set(balance)
		}
		if amount.IsZero() {
			continue
		}
		statedb.SubBalance(d.Address, amount, tracing.BalanceChangeUnspecified)
		taken.Add(taken, amount)
	}
	statedb.AddBalance(r.Recipient, taken, tracing.BalanceChangeUnspecified)

	for _, w := range r.StorageWrites {
		statedb.SetState(w.Address, w.Slot, w.Value)
	}
	log.Info("Applied recovery fork", "block", r.BlockNumber, "recipient", r.Recipient, "credited", taken.Dec(), "expected", r.ExpectedTotal.Dec())
}

// SetRecoveryForkForTesting replaces the active recovery configuration and
// returns a function restoring the previous one. Only for tests.
func SetRecoveryForkForTesting(r *RecoveryForkConfig) (restore func()) {
	if err := r.Validate(); err != nil {
		panic(err)
	}
	prev := activeRecovery
	activeRecovery = r
	return func() { activeRecovery = prev }
}
