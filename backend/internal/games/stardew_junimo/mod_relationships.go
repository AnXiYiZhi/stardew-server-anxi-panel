package stardew_junimo

import (
	"fmt"
	"sort"
	"strings"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

type modRelationshipIndex struct {
	mods     []registry.ModInfo
	byFolder map[string]int
	byUnique map[string]int
	deps     map[int][]int
	revDeps  map[int][]int
	bundles  map[string][]int
}

func buildModRelationshipIndex(mods []registry.ModInfo) modRelationshipIndex {
	idx := modRelationshipIndex{
		mods:     mods,
		byFolder: map[string]int{},
		byUnique: map[string]int{},
		deps:     map[int][]int{},
		revDeps:  map[int][]int{},
		bundles:  map[string][]int{},
	}
	for i, mod := range mods {
		idx.byFolder[strings.ToLower(mod.FolderName)] = i
		if uniqueID := normalizeModUniqueID(mod.UniqueID); uniqueID != "" {
			idx.byUnique[uniqueID] = i
		}
		if key := modNexusPackageBundleKey(mod); key != "" {
			idx.bundles[key] = append(idx.bundles[key], i)
		}
	}
	for i, mod := range mods {
		for _, dep := range mod.Dependencies {
			if !dep.Required {
				continue
			}
			depIndex, ok := idx.byUnique[normalizeModUniqueID(dep.UniqueID)]
			if !ok || depIndex == i {
				continue
			}
			idx.deps[i] = appendUniqueIndex(idx.deps[i], depIndex)
			idx.revDeps[depIndex] = appendUniqueIndex(idx.revDeps[depIndex], i)
		}
	}
	return idx
}

func (idx modRelationshipIndex) resolve(modID string) (int, error) {
	modID = strings.TrimSpace(modID)
	if modID == "" {
		return -1, fmt.Errorf("mod id is required")
	}
	if i, ok := idx.byFolder[strings.ToLower(modID)]; ok {
		return i, nil
	}
	if i, ok := idx.byUnique[normalizeModUniqueID(modID)]; ok {
		return i, nil
	}
	return -1, fmt.Errorf("Mod %q does not exist", modID)
}

func (idx modRelationshipIndex) dependencyClosure(seed int) []int {
	return idx.walk(seed, true, false)
}

func (idx modRelationshipIndex) connectedClosure(seed int) []int {
	seen := map[int]bool{}
	queue := []int{seed}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current < 0 || current >= len(idx.mods) || seen[current] {
			continue
		}
		seen[current] = true
		neighbors := append([]int{}, idx.deps[current]...)
		neighbors = append(neighbors, idx.revDeps[current]...)
		for _, next := range idx.bundles[modNexusPackageBundleKey(idx.mods[current])] {
			neighbors = appendUniqueIndex(neighbors, next)
		}
		for _, next := range neighbors {
			if !seen[next] {
				queue = append(queue, next)
			}
		}
	}
	return sortedRelationshipIndexes(seen, idx.mods)
}

func (idx modRelationshipIndex) enableClosure(seed int) []int {
	return idx.walk(seed, true, true)
}

func (idx modRelationshipIndex) disableClosure(seed int) []int {
	return idx.walk(seed, false, true)
}

func (idx modRelationshipIndex) walk(seed int, followDependencies bool, includeBundle bool) []int {
	seen := map[int]bool{}
	queue := []int{seed}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current < 0 || current >= len(idx.mods) || seen[current] {
			continue
		}
		seen[current] = true
		neighbors := idx.revDeps[current]
		if followDependencies {
			neighbors = idx.deps[current]
		}
		for _, next := range neighbors {
			if !seen[next] {
				queue = append(queue, next)
			}
		}
		if includeBundle {
			for _, next := range idx.bundles[modNexusPackageBundleKey(idx.mods[current])] {
				if !seen[next] {
					queue = append(queue, next)
				}
			}
		}
	}
	return sortedRelationshipIndexes(seen, idx.mods)
}

func sortedRelationshipIndexes(seen map[int]bool, mods []registry.ModInfo) []int {
	indexes := make([]int, 0, len(seen))
	for i := range seen {
		indexes = append(indexes, i)
	}
	sort.SliceStable(indexes, func(i, j int) bool {
		a := mods[indexes[i]]
		b := mods[indexes[j]]
		if a.FolderName == b.FolderName {
			return a.UniqueID < b.UniqueID
		}
		return a.FolderName < b.FolderName
	})
	return indexes
}

func normalizeModUniqueID(uniqueID string) string {
	return strings.ToLower(strings.TrimSpace(uniqueID))
}

func modNexusPackageBundleKey(mod registry.ModInfo) string {
	if mod.OriginSource == "nexus" && mod.OriginNexusModID > 0 {
		return fmt.Sprintf("nexus:%d", mod.OriginNexusModID)
	}
	if mod.NexusModID > 0 {
		return fmt.Sprintf("nexus:%d", mod.NexusModID)
	}
	return ""
}

func appendUniqueIndex(indexes []int, value int) []int {
	for _, existing := range indexes {
		if existing == value {
			return indexes
		}
	}
	return append(indexes, value)
}

func modCanRelationshipToggle(mod registry.ModInfo) bool {
	return !mod.BuiltIn && !isSMAPIRuntimeMod(mod) && !isControlModInfo(mod)
}
