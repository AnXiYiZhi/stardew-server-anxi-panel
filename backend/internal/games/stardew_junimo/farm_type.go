package stardew_junimo

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type NewGameFarmType struct {
	ID              string
	Builtin         bool
	BuiltinNumber   int
	CompatibilityID bool
}

var builtinFarmTypeNames = []string{
	"standard", "riverland", "forest", "hilltop", "wilderness", "fourcorners", "beach", "meadowlands",
}

func NormalizeNewGameFarmType(value string) (NewGameFarmType, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		raw = "standard"
	}
	if number, err := strconv.Atoi(raw); err == nil {
		if number < 0 || number >= len(builtinFarmTypeNames) {
			return NewGameFarmType{}, fmt.Errorf("farmType 数字必须在 0~7 之间")
		}
		return NewGameFarmType{ID: builtinFarmTypeNames[number], Builtin: true, BuiltinNumber: number}, nil
	}

	alias := strings.ToLower(raw)
	alias = strings.NewReplacer(" ", "", "\t", "", "_", "", "-", "").Replace(alias)
	aliases := map[string]int{
		"standard": 0, "standardfarm": 0,
		"riverland": 1, "riverlandfarm": 1, "river": 1,
		"forest": 2, "forestfarm": 2,
		"hilltop": 3, "hilltopfarm": 3, "hills": 3,
		"wilderness": 4, "wildernessfarm": 4,
		"fourcorners": 5, "fourcornersfarm": 5,
		"beach": 6, "beachfarm": 6,
		"meadowlands": 7, "meadowlandsfarm": 7,
	}
	if number, ok := aliases[alias]; ok {
		return NewGameFarmType{ID: builtinFarmTypeNames[number], Builtin: true, BuiltinNumber: number}, nil
	}
	if strings.EqualFold(raw, "modded") {
		return NewGameFarmType{ID: "modded", CompatibilityID: true}, nil
	}
	if !utf8.ValidString(raw) || len(raw) > farmCatalogMaxIDBytes {
		return NewGameFarmType{}, fmt.Errorf("farmType Id 不是有效 UTF-8 或超过 %d 字节", farmCatalogMaxIDBytes)
	}
	for _, r := range raw {
		if unicode.IsControl(r) {
			return NewGameFarmType{}, fmt.Errorf("farmType Id 包含控制字符")
		}
	}
	return NewGameFarmType{ID: raw}, nil
}

func isBuiltinFarmType(farmType string) bool {
	normalized, err := NormalizeNewGameFarmType(farmType)
	return err == nil && normalized.Builtin
}

func farmTypeServerValue(farmType string) (any, error) {
	normalized, err := NormalizeNewGameFarmType(farmType)
	if err != nil {
		return nil, err
	}
	if normalized.Builtin {
		return normalized.BuiltinNumber, nil
	}
	return normalized.ID, nil
}
