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

package simulated

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/misc"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
)

// TestRecoveryForkBuildAndImport runs the fork block through the real
// block-building path (engine API -> miner) and the import path (newPayload ->
// state processor). If the two disagree, the built block is rejected and the
// chain stops advancing.
func TestRecoveryForkBuildAndImport(t *testing.T) {
	var (
		wallet  = common.HexToAddress("0xb41d6e00000000000000000000000000000000b1")
		debtorA = common.HexToAddress("0xa000000000000000000000000000000000000001")
		debtorB = common.HexToAddress("0xa000000000000000000000000000000000000002")
		ctx     = context.Background()
	)
	defer misc.SetRecoveryForkForTesting(&misc.RecoveryForkConfig{
		ChainID:     params.AllDevChainProtocolChanges.ChainID.Uint64(),
		BlockNumber: 2,
		Recipient:   wallet,
		Debits: []misc.RecoveryDebit{
			{Address: debtorA, Amount: uint256.NewInt(600)},
			{Address: debtorB, Amount: uint256.NewInt(50)},
		},
		ExpectedTotal: uint256.NewInt(650),
	})()

	sim := NewBackend(types.GenesisAlloc{
		debtorA: {Balance: big.NewInt(1000)},
		debtorB: {Balance: big.NewInt(20)},
		wallet:  {Balance: big.NewInt(1)},
	})
	defer sim.Close()
	client := sim.Client()

	for i := 0; i < 3; i++ {
		sim.Commit()
	}
	head, err := client.BlockNumber(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if head != 3 {
		t.Fatalf("chain head %d, want 3 (fork block rejected?)", head)
	}
	balance := func(addr common.Address, number int64) uint64 {
		t.Helper()
		bal, err := client.BalanceAt(ctx, addr, big.NewInt(number))
		if err != nil {
			t.Fatal(err)
		}
		return bal.Uint64()
	}
	for _, c := range []struct {
		addr   common.Address
		number int64
		want   uint64
	}{
		{debtorA, 1, 1000}, {debtorB, 1, 20}, {wallet, 1, 1},
		{debtorA, 2, 400}, {debtorB, 2, 0}, {wallet, 2, 621},
		{debtorA, 3, 400}, {debtorB, 3, 0}, {wallet, 3, 621},
	} {
		if got := balance(c.addr, c.number); got != c.want {
			t.Errorf("block %d, %s: balance %d, want %d", c.number, c.addr, got, c.want)
		}
	}
}
