// SPDX-License-Identifier: AGPL-3.0-or-later

package stats

import (
	"testing"

	statsService "github.com/xtls/xray-core/app/stats/command"
)

func TestParseStats(t *testing.T) {
	stats := []*statsService.Stat{
		{Name: "user>>>alice@gopanel>>>traffic>>>uplink", Value: 1000},
		{Name: "user>>>alice@gopanel>>>traffic>>>downlink", Value: 2000},
		{Name: "user>>>bob@gopanel>>>traffic>>>uplink", Value: 500},
		{Name: "inbound>>>vless-in>>>traffic>>>uplink", Value: 9999},  // не user, игнор
		{Name: "user>>>charlie@gopanel>>>traffic>>>uplink", Value: 0}, // ноль
	}

	out := parseStats(stats)

	if out["alice@gopanel"].up != 1000 || out["alice@gopanel"].down != 2000 {
		t.Errorf("alice: got up=%d down=%d", out["alice@gopanel"].up, out["alice@gopanel"].down)
	}
	if out["bob@gopanel"].up != 500 {
		t.Errorf("bob: got up=%d", out["bob@gopanel"].up)
	}
	if _, ok := out["charlie@gopanel"]; !ok {
		t.Error("charlie: expected to be present (with zero), got missing")
	}
	if _, ok := out["vless-in"]; ok {
		t.Error("inbound stat should be ignored")
	}
}
