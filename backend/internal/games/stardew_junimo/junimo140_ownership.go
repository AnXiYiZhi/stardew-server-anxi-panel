package stardew_junimo

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

type junimoFarmhandOwnershipRecord struct{ Platform, Id, Origin string }

// The Control receipt proves the live Junimo map and its disk write. Compare
// that receipt against every farmhand identity in the stable saved XML too;
// the same number of released records for different roles is insufficient.
func verifyImportFarmhandOwnershipStore(dataDir, saveName string, expected int) error {
	if expected <= 0 || saveName == "" || saveName == "." || saveName == ".." || strings.ContainsAny(saveName, `/\`) {
		return errors.New("invalid farmhand ownership verification target")
	}
	dir := filepath.Join(savesDir(dataDir), "Saves", saveName)
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("farmhand ownership save directory is unavailable")
	}
	mainPath := filepath.Join(dir, saveName)
	if info, err := os.Lstat(mainPath); err != nil || !info.Mode().IsRegular() {
		return errors.New("farmhand ownership save file is unavailable")
	}
	roster, err := readSaveRosterSnapshot(mainPath)
	if err != nil || len(roster.farmhands) != expected {
		return errors.New("farmhand ownership role count disagrees with saved XML")
	}
	records, err := readJunimoFarmhandOwnershipStore(dir, false)
	if err != nil {
		return err
	}
	ids := map[string]bool{}
	for _, farmer := range roster.farmhands {
		id := strings.TrimSpace(farmer.UniqueMultiplayerID)
		if id == "" {
			id = strings.TrimSpace(farmer.UniqueMultiplayerIDFallback)
		}
		value, parseErr := strconv.ParseInt(id, 10, 64)
		if parseErr != nil || value == 0 || strconv.FormatInt(value, 10) != id || ids[id] {
			return errors.New("saved farmhand identity is invalid or duplicated")
		}
		ids[id] = true
		record, ok := records[id]
		if !ok || record.Platform != "released" || record.Id != "" || record.Origin != "operator" {
			return errors.New("saved farmhand ownership is not durably released")
		}
	}
	return nil
}

func readJunimoFarmhandOwnershipStore(dir string, allowMissing bool) (map[string]junimoFarmhandOwnershipRecord, error) {
	invalid := errors.New("farmhand ownership store is invalid or unavailable")
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, invalid
	}
	path := filepath.Join(dir, "farmhand-ownership.json")
	info, err = os.Lstat(path)
	if allowMissing && errors.Is(err, os.ErrNotExist) {
		return map[string]junimoFarmhandOwnershipRecord{}, nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Size() > 1024*1024 {
		return nil, invalid
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, invalid
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, 1024*1024+1))
	if err != nil || len(raw) > 1024*1024 || !utf8.Valid(raw) {
		return nil, invalid
	}
	if uniqueJunimo140GroupJSONKeys(json.NewDecoder(bytes.NewReader(raw)), 0) != nil {
		return nil, invalid
	}
	var root map[string]json.RawMessage
	if json.Unmarshal(raw, &root) != nil || len(root) != 1 || root["Farmhands"] == nil {
		return nil, invalid
	}
	var encoded map[string]json.RawMessage
	if json.Unmarshal(root["Farmhands"], &encoded) != nil || encoded == nil {
		return nil, invalid
	}
	records := make(map[string]junimoFarmhandOwnershipRecord, len(encoded))
	for key, value := range encoded {
		id, err := strconv.ParseInt(key, 10, 64)
		if err != nil || id == 0 || strconv.FormatInt(id, 10) != key {
			return nil, invalid
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal(value, &fields) != nil || len(fields) < 2 || len(fields) > 3 {
			return nil, invalid
		}
		for field := range fields {
			if field != "Platform" && field != "Id" && field != "Origin" {
				return nil, invalid
			}
		}
		record := junimoFarmhandOwnershipRecord{Origin: "claim"}
		for _, field := range []struct {
			key    string
			target *string
		}{{"Platform", &record.Platform}, {"Id", &record.Id}, {"Origin", &record.Origin}} {
			data, ok := fields[field.key]
			if !ok && field.key == "Origin" {
				continue
			} // fixed upstream's legacy default
			if !ok || bytes.Equal(bytes.TrimSpace(data), []byte("null")) || json.Unmarshal(data, field.target) != nil {
				return nil, invalid
			}
		}
		records[key] = record
	}
	return records, nil
}

func verifyDeletedJunimoFarmhandOwnership(dataDir, saveName, playerID string) error {
	contract, err := readConsoleRuntimeContract(dataDir)
	if err != nil {
		return err
	}
	if contract.fifo != junimo140InputFIFO {
		return nil
	}
	id, err := strconv.ParseInt(playerID, 10, 64)
	if err != nil || id == 0 || strconv.FormatInt(id, 10) != playerID || saveName == "" || saveName == "." || saveName == ".." || strings.ContainsAny(saveName, `/\`) {
		return errors.New("invalid deleted farmhand verification target")
	}
	records, err := readJunimoFarmhandOwnershipStore(filepath.Join(savesDir(dataDir), "Saves", saveName), true)
	if err != nil {
		return err
	}
	if _, present := records[playerID]; present {
		return errors.New("deleted farmhand ownership is still persisted")
	}
	return nil
}
