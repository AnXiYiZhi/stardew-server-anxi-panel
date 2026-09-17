package web

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

type resourceVolumeReader interface {
	ResourceVolumeSizes(context.Context, string, string, []string) (map[string]int64, error)
}

type resourceStorageSnapshot struct {
	worlds, games map[string]*int64
	timestamp     string
	expires       time.Time
}

func (s *server) resourceStorage(_ context.Context) resourceStorageSnapshot {
	s.resourceStorageMu.Lock()
	defer s.resourceStorageMu.Unlock()
	if !s.resourceStorageRefreshing && !time.Now().Before(s.resourceStorageCache.expires) {
		s.resourceStorageRefreshing = true
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			result := s.collectResourceStorage(ctx)
			s.resourceStorageMu.Lock()
			defer s.resourceStorageMu.Unlock()
			s.resourceStorageCache = result
			s.resourceStorageRefreshing = false
		}()
	}
	// Return the last completed sample immediately, including on a cold start.
	// Maps are immutable after publication; CPU/memory never wait on a disk walk.
	return s.resourceStorageCache
}

func (s *server) collectResourceStorage(ctx context.Context) resourceStorageSnapshot {
	result := resourceStorageSnapshot{worlds: map[string]*int64{}, games: map[string]*int64{}, timestamp: time.Now().UTC().Format(time.RFC3339), expires: time.Now().Add(time.Minute)}
	if s.store == nil || s.registry == nil {
		return result
	}
	instances, err := s.store.ListInstances(ctx)
	if err != nil {
		return result
	}
	plans := make(map[string]registry.ResourceStorage)
	gamePlans := make(map[string]registry.ResourceStorage)
	invalidGames := make(map[string]bool)
	volumeImages := map[string]string{}
	for _, instance := range instances {
		driver, err := s.registry.Get(instance.DriverID)
		if err != nil {
			invalidGames[instance.DriverID] = true
			continue
		}
		provider, ok := driver.(registry.ResourceStorageProvider)
		if !ok {
			invalidGames[instance.DriverID] = true
			continue
		}
		plan, err := provider.ResourceStorage(makeRegistryInstance(instance))
		if err != nil {
			invalidGames[instance.DriverID] = true
			continue
		}
		plans[instance.ID] = plan
		game := gamePlans[instance.DriverID]
		game.Directories = append(game.Directories, plan.Directories...)
		game.Directories = append(game.Directories, plan.SharedDirectories...)
		game.Volumes = append(game.Volumes, plan.Volumes...)
		game.Volumes = append(game.Volumes, plan.SharedVolumes...)
		gamePlans[instance.DriverID] = game
		for _, name := range append(append([]string{}, plan.Volumes...), plan.SharedVolumes...) {
			if _, exists := volumeImages[name]; !exists {
				volumeImages[name] = plan.ProbeImage
			}
		}
	}
	volumes := map[string]int64{}
	groups := map[string][]string{}
	for name, image := range volumeImages {
		volumes[name] = -1
		groups[image] = append(groups[image], name)
	}
	if reader, ok := s.docker.(resourceVolumeReader); ok {
		for image, names := range groups {
			if sizes, err := reader.ResourceVolumeSizes(ctx, s.config.DataDir, image, names); err == nil {
				for _, name := range names {
					if size, found := sizes[name]; found {
						volumes[name] = size
					}
				}
			}
		}
	}
	// Cache individual directory walks too: world and game views share them.
	directories := make(map[string]*int64)
	measure := func(plan registry.ResourceStorage) *int64 {
		var total int64
		for _, path := range uniqueResourceRoots(plan.Directories) {
			value, exists := directories[path]
			if !exists {
				value = measureResourceDirectory(ctx, path)
				directories[path] = value
			}
			if value == nil {
				return nil
			}
			total += *value
		}
		seen := map[string]bool{}
		for _, name := range plan.Volumes {
			if seen[name] {
				continue
			}
			seen[name] = true
			if volumes == nil || volumes[name] < 0 {
				return nil
			}
			total += volumes[name]
		}
		return &total
	}
	for id, plan := range plans {
		result.worlds[id] = measure(plan)
	}
	for id, plan := range gamePlans {
		if !invalidGames[id] {
			result.games[id] = measure(plan)
		}
	}
	result.expires = time.Now().Add(time.Minute)
	return result
}

func uniqueResourceRoots(paths []string) []string {
	clean := make([]string, 0, len(paths))
	for _, path := range paths {
		if filepath.IsAbs(path) {
			clean = append(clean, filepath.Clean(path))
		}
	}
	sort.Slice(clean, func(i, j int) bool { return len(clean[i]) < len(clean[j]) })
	var result []string
	for _, path := range clean {
		contained := false
		for _, root := range result {
			rel, err := filepath.Rel(root, path)
			if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				contained = true
				break
			}
		}
		if !contained {
			result = append(result, path)
		}
	}
	return result
}

func measureResourceDirectory(ctx context.Context, path string) *int64 {
	var total int64
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return &total
	}
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	err = filepath.WalkDir(path, func(_ string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		// Never follow symlinks into other worlds or external host directories.
		if !entry.Type().IsRegular() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	if err != nil {
		return nil
	}
	return &total
}
