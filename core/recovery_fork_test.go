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

package core

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/beacon"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/consensus/misc"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
)

var (
	recoveryWallet  = common.HexToAddress("0xb41d6e00000000000000000000000000000000b1")
	recoveryDebtorA = common.HexToAddress("0xa000000000000000000000000000000000000001")
	recoveryDebtorB = common.HexToAddress("0xa000000000000000000000000000000000000002")
)

const recoveryForkBlock = 3

func recoveryTestGenesis() *Genesis {
	config := *params.MergedTestChainConfig
	return &Genesis{
		Config: &config,
		Alloc: types.GenesisAlloc{
			recoveryDebtorA:                  {Balance: big.NewInt(1000)},
			recoveryDebtorB:                  {Balance: big.NewInt(20)}, // less than planned
			recoveryWallet:                   {Balance: big.NewInt(1)},
			params.HistoryStorageAddress:     {Code: params.HistoryStorageCode},
			params.WithdrawalQueueAddress:    {Code: params.WithdrawalQueueCode},
			params.ConsolidationQueueAddress: {Code: params.ConsolidationQueueCode},
		},
	}
}

func recoveryTestConfig(chainID uint64) *misc.RecoveryForkConfig {
	return &misc.RecoveryForkConfig{
		ChainID:     chainID,
		BlockNumber: recoveryForkBlock,
		Recipient:   recoveryWallet,
		Debits: []misc.RecoveryDebit{
			{Address: recoveryDebtorA, Amount: uint256.NewInt(600)},
			{Address: recoveryDebtorB, Amount: uint256.NewInt(50)},
		},
		ExpectedTotal: uint256.NewInt(650),
	}
}

// TestRecoveryForkChain checks that a chain built by an upgraded client is
// imported by an upgraded client, with the balances moved exactly at the fork
// block.
func TestRecoveryForkChain(t *testing.T) {
	gspec := recoveryTestGenesis()
	defer misc.SetRecoveryForkForTesting(recoveryTestConfig(gspec.Config.ChainID.Uint64()))()

	engine := beacon.New(ethash.NewFaker())
	_, blocks, _ := GenerateChainWithGenesis(gspec, engine, 5, nil)

	chain, err := NewBlockChain(rawdb.NewMemoryDatabase(), nil, gspec, nil, engine, vm.Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer chain.Stop()
	if n, err := chain.InsertChain(blocks); err != nil {
		t.Fatalf("block %d: import failed: %v", n, err)
	}
	check := func(number uint64, want map[common.Address]uint64) {
		t.Helper()
		statedb, err := chain.StateAt(chain.GetBlockByNumber(number).Root())
		if err != nil {
			t.Fatal(err)
		}
		for addr, bal := range want {
			if got := statedb.GetBalance(addr).Uint64(); got != bal {
				t.Errorf("block %d, %s: balance %d, want %d", number, addr, got, bal)
			}
		}
	}
	before := map[common.Address]uint64{recoveryDebtorA: 1000, recoveryDebtorB: 20, recoveryWallet: 1}
	after := map[common.Address]uint64{recoveryDebtorA: 400, recoveryDebtorB: 0, recoveryWallet: 1 + 600 + 20}
	check(recoveryForkBlock-1, before)
	check(recoveryForkBlock, after)
	check(recoveryForkBlock+2, after) // applied once, not on every block
}

// TestRecoveryForkSplitsOldClients checks that upgraded and non-upgraded
// clients reject each other's fork block (and only that block).
func TestRecoveryForkSplitsOldClients(t *testing.T) {
	gspec := recoveryTestGenesis()
	engine := beacon.New(ethash.NewFaker())
	fork := recoveryTestConfig(gspec.Config.ChainID.Uint64())

	// Chain built by an old client, imported by an upgraded one.
	_, oldBlocks, _ := GenerateChainWithGenesis(gspec, engine, 5, nil)
	restore := misc.SetRecoveryForkForTesting(fork)
	chain, _ := NewBlockChain(rawdb.NewMemoryDatabase(), nil, gspec, nil, engine, vm.Config{}, nil)
	n, err := chain.InsertChain(oldBlocks)
	chain.Stop()
	if err == nil || n != recoveryForkBlock-1 {
		t.Errorf("upgraded client: want rejection at block index %d, got index %d, err %v", recoveryForkBlock-1, n, err)
	}

	// Chain built by an upgraded client, imported by an old one.
	_, newBlocks, _ := GenerateChainWithGenesis(gspec, engine, 5, nil)
	restore()
	chain, _ = NewBlockChain(rawdb.NewMemoryDatabase(), nil, gspec, nil, engine, vm.Config{}, nil)
	n, err = chain.InsertChain(newBlocks)
	chain.Stop()
	if err == nil || n != recoveryForkBlock-1 {
		t.Errorf("old client: want rejection at block index %d, got index %d, err %v", recoveryForkBlock-1, n, err)
	}
}

// TestRecoveryForkOtherChain checks that the fork does nothing on a chain with
// a different chain ID.
func TestRecoveryForkOtherChain(t *testing.T) {
	gspec := recoveryTestGenesis()
	defer misc.SetRecoveryForkForTesting(recoveryTestConfig(gspec.Config.ChainID.Uint64() + 1))()

	engine := beacon.New(ethash.NewFaker())
	_, blocks, _ := GenerateChainWithGenesis(gspec, engine, 5, nil)
	chain, _ := NewBlockChain(rawdb.NewMemoryDatabase(), nil, gspec, nil, engine, vm.Config{}, nil)
	defer chain.Stop()
	if _, err := chain.InsertChain(blocks); err != nil {
		t.Fatal(err)
	}
	statedb, _ := chain.StateAt(chain.CurrentBlock().Root)
	if got := statedb.GetBalance(recoveryDebtorA).Uint64(); got != 1000 {
		t.Errorf("debtor balance changed on another chain: %d", got)
	}
}
