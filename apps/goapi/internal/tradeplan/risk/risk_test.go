package risk

import (
	"math"
	"testing"
)

func TestParseDirection(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		in   string
		want Direction
		ok   bool
	}{
		{"LONG", DirectionLong, true},
		{"long", DirectionLong, true},
		{"SHORT", DirectionShort, true},
		{"", "", false},
		{"FLAT", "", false},
	} {
		got, err := ParseDirection(tc.in)
		if tc.ok && err != nil {
			t.Fatalf("ParseDirection(%q): %v", tc.in, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("ParseDirection(%q): want error", tc.in)
		}
		if tc.ok && got != tc.want {
			t.Fatalf("ParseDirection(%q) = %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestValidateGeometry_Long_ok(t *testing.T) {
	t.Parallel()
	if err := ValidateGeometry(DirectionLong, 100, 97, 109); err != nil {
		t.Fatal(err)
	}
}

func TestValidateGeometry_Long_bad(t *testing.T) {
	t.Parallel()
	if err := ValidateGeometry(DirectionLong, 100, 101, 109); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateGeometry_Short_ok(t *testing.T) {
	t.Parallel()
	if err := ValidateGeometry(DirectionShort, 100, 103, 97); err != nil {
		t.Fatal(err)
	}
}

func TestRiskRewardPerUnit_Long(t *testing.T) {
	t.Parallel()
	risk, reward, rr, err := RiskRewardPerUnit(DirectionLong, 100, 97, 109)
	if err != nil {
		t.Fatal(err)
	}
	if risk != 3 || reward != 9 {
		t.Fatalf("risk=%v reward=%v", risk, reward)
	}
	if math.Abs(rr-3) > 1e-9 {
		t.Fatalf("rr=%v want 3", rr)
	}
}

func TestMaxLoss(t *testing.T) {
	t.Parallel()
	if v := MaxLoss(2.5, 10); v != 25 {
		t.Fatalf("got %v", v)
	}
}
