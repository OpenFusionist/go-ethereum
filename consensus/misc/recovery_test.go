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

package misc_test

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/misc"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
)

var (
	testWallet   = common.HexToAddress("0x5afe00000000000000000000000000000000c0de")
	testContract = common.HexToAddress("0xc000000000000000000000000000000000000001")
	testDebtorA  = common.HexToAddress("0xa000000000000000000000000000000000000001")
	testDebtorB  = common.HexToAddress("0xa000000000000000000000000000000000000002")
)

func testRecoveryConfig() *misc.RecoveryForkConfig {
	return &misc.RecoveryForkConfig{
		ChainID:     648,
		BlockNumber: 100,
		Recipient:   testWallet,
		Debits: []misc.RecoveryDebit{
			{Address: testDebtorA, Amount: uint256.NewInt(70)},
			{Address: testDebtorB, Amount: uint256.NewInt(30)},
		},
		ExpectedTotal: uint256.NewInt(100),
	}
}

func TestRecoveryValidate(t *testing.T) {
	if err := misc.EnduranceRecovery.Validate(); err != nil {
		t.Fatalf("shipped configuration invalid: %v", err)
	}
	if err := testRecoveryConfig().Validate(); err != nil {
		t.Fatalf("valid configuration rejected: %v", err)
	}
	cases := map[string]func(c *misc.RecoveryForkConfig){
		"no recipient":      func(c *misc.RecoveryForkConfig) { c.Recipient = common.Address{} },
		"no debits":         func(c *misc.RecoveryForkConfig) { c.Debits = nil },
		"total mismatch":    func(c *misc.RecoveryForkConfig) { c.ExpectedTotal = uint256.NewInt(99) },
		"nil total":         func(c *misc.RecoveryForkConfig) { c.ExpectedTotal = nil },
		"zero amount":       func(c *misc.RecoveryForkConfig) { c.Debits[0].Amount = new(uint256.Int) },
		"recipient debited": func(c *misc.RecoveryForkConfig) { c.Debits[0].Address = testWallet },
		"duplicate":         func(c *misc.RecoveryForkConfig) { c.Debits[1].Address = testDebtorA },
	}
	for name, mutate := range cases {
		c := testRecoveryConfig()
		mutate(c)
		if err := c.Validate(); err == nil {
			t.Errorf("%s: expected validation error", name)
		}
	}
}

// Check the compiled token writes against the two public getter values at the anchor.
func TestRecoveryFameBurn(t *testing.T) {
	token := common.HexToAddress("0x0466E0F1Ee4cF3cA2873af7C3690502150516731")
	holder := common.HexToAddress("0x1558ad8ed679d00f7c419291b4b32da1a59b06e8")
	// Solidity lays out _balances at slot 0 and _totalSupply at slot 2.
	balanceSlot := crypto.Keccak256Hash(common.LeftPadBytes(holder.Bytes(), 32), make([]byte, 32))
	supplySlot := common.BigToHash(big.NewInt(2))
	before := map[common.Hash]*uint256.Int{
		balanceSlot: uint256.MustFromDecimal("2000003191111535819033305"),
		supplySlot:  uint256.MustFromDecimal("2027256410000000000000000"),
	}
	burn := uint256.MustFromDecimal("2000000000000000000000000")
	for _, write := range misc.EnduranceRecovery.StorageWrites {
		if write.Address != token {
			continue
		}
		initial, ok := before[write.Slot]
		if !ok {
			t.Fatalf("unexpected or repeated FAME write at %v", write.Slot)
		}
		if want := new(uint256.Int).Sub(initial, burn).Bytes32(); write.Value != want {
			t.Errorf("FAME slot %v: got %x, want %x", write.Slot, write.Value, want)
		}
		delete(before, write.Slot)
	}
	if len(before) != 0 {
		t.Fatalf("missing FAME writes: %v", before)
	}
}

func TestIsRecoveryBlock(t *testing.T) {
	defer misc.SetRecoveryForkForTesting(testRecoveryConfig())()

	endurance := &params.ChainConfig{ChainID: big.NewInt(648)}
	other := &params.ChainConfig{ChainID: big.NewInt(1)}

	if !misc.IsRecoveryBlock(endurance, 100) {
		t.Error("fork block not detected")
	}
	for _, n := range []uint64{0, 99, 101} {
		if misc.IsRecoveryBlock(endurance, n) {
			t.Errorf("block %d wrongly detected as fork block", n)
		}
	}
	if misc.IsRecoveryBlock(other, 100) {
		t.Error("fork applied on a different chain")
	}
}

func TestApplyRecoveryHardFork(t *testing.T) {
	defer misc.SetRecoveryForkForTesting(testRecoveryConfig())()

	statedb, err := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
	if err != nil {
		t.Fatal(err)
	}
	statedb.AddBalance(testDebtorA, uint256.NewInt(500), tracing.BalanceChangeUnspecified)
	statedb.AddBalance(testDebtorB, uint256.NewInt(10), tracing.BalanceChangeUnspecified) // less than the 30 planned
	statedb.AddBalance(testWallet, uint256.NewInt(5), tracing.BalanceChangeUnspecified)

	misc.ApplyRecoveryHardFork(statedb)

	want := map[common.Address]uint64{
		testDebtorA: 430,         // 500 - 70
		testDebtorB: 0,           // capped: only 10 available
		testWallet:  5 + 70 + 10, // credited with what was actually taken
	}
	for addr, bal := range want {
		if got := statedb.GetBalance(addr); got.Uint64() != bal {
			t.Errorf("%s: balance %d, want %d", addr, got.Uint64(), bal)
		}
	}
}

func TestApplyRecoveryFreshWallet(t *testing.T) {
	defer misc.SetRecoveryForkForTesting(testRecoveryConfig())()

	// The recipient has never been used on chain before the fork.
	statedb, _ := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
	statedb.AddBalance(testDebtorA, uint256.NewInt(100), tracing.BalanceChangeUnspecified)
	statedb.AddBalance(testDebtorB, uint256.NewInt(100), tracing.BalanceChangeUnspecified)

	misc.ApplyRecoveryHardFork(statedb)

	if got := statedb.GetBalance(testWallet).Uint64(); got != 100 {
		t.Errorf("fresh recipient balance %d, want 100", got)
	}
}

func TestApplyRecoveryStorageWrites(t *testing.T) {
	c := testRecoveryConfig()
	slot := common.HexToHash("0x03")
	c.StorageWrites = []misc.RecoveryStorageWrite{{Address: testContract, Slot: slot, Value: common.HexToHash("0x2a")}}
	defer misc.SetRecoveryForkForTesting(c)()

	statedb, _ := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
	statedb.AddBalance(testDebtorA, uint256.NewInt(100), tracing.BalanceChangeUnspecified)
	statedb.AddBalance(testDebtorB, uint256.NewInt(100), tracing.BalanceChangeUnspecified)

	misc.ApplyRecoveryHardFork(statedb)

	if got := statedb.GetState(testContract, slot); got != common.HexToHash("0x2a") {
		t.Errorf("storage slot = %x, want 0x2a", got)
	}
}
