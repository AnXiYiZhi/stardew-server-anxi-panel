package stardew_junimo

import "testing"

func TestNewGameFarmTypeNormalization(t *testing.T) {
	tests := []struct {
		input   string
		id      string
		builtin bool
		number  int
	}{
		{"0", "standard", true, 0}, {"7", "meadowlands", true, 7},
		{" Standard Farm ", "standard", true, 0}, {"FOUR-CORNERS", "fourcorners", true, 5},
		{"MeadowlandsFarm", "meadowlands", true, 7}, {"FrontierFarm", "FrontierFarm", false, 0},
		{"UnknownExplicitId", "UnknownExplicitId", false, 0}, {"modded", "modded", false, 0},
	}
	for _, tc := range tests {
		got, err := NormalizeNewGameFarmType(tc.input)
		if err != nil {
			t.Fatalf("%q: %v", tc.input, err)
		}
		if got.ID != tc.id || got.Builtin != tc.builtin || got.BuiltinNumber != tc.number {
			t.Fatalf("%q => %#v", tc.input, got)
		}
	}
}

func TestNewGameFarmTypeAllBuiltinNamesAndAliases(t *testing.T) {
	tests := []struct {
		input  string
		id     string
		number int
	}{
		{"Standard", "standard", 0},
		{"River Land Farm", "riverland", 1},
		{"Forest Farm", "forest", 2},
		{"Hill-Top", "hilltop", 3},
		{"Wilderness_Farm", "wilderness", 4},
		{"Four Corners", "fourcorners", 5},
		{"FourCornersFarm", "fourcorners", 5},
		{"Beach Farm", "beach", 6},
		{"Meadowlands Farm", "meadowlands", 7},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := NormalizeNewGameFarmType(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			if !got.Builtin || got.ID != tc.id || got.BuiltinNumber != tc.number {
				t.Fatalf("%q => %#v", tc.input, got)
			}
			serverValue, err := farmTypeServerValue(tc.input)
			if err != nil || serverValue != tc.number {
				t.Fatalf("farmTypeServerValue(%q) = %#v, %v", tc.input, serverValue, err)
			}
		})
	}
}

func TestNewGameFarmTypeRejectsOutOfRangeAndInvalid(t *testing.T) {
	for _, value := range []string{"8", "99", "-1", string([]byte{0xff}), "Bad\nFarm"} {
		if _, err := NormalizeNewGameFarmType(value); err == nil {
			t.Fatalf("%q accepted", value)
		}
	}
}
