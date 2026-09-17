package registry

// ResourceStorageProvider describes owned storage without exposing paths in API responses.
// The collector counts shared roots/volumes once per game. Container images are
// Docker infrastructure, not world data; mutable game installations are volumes.
type ResourceStorageProvider interface {
	ResourceStorage(instance Instance) (ResourceStorage, error)
}

type ResourceStorage struct {
	Directories, Volumes             []string
	SharedDirectories, SharedVolumes []string
	// An already-installed runtime image with POSIX sh/find/stat/awk.
	ProbeImage string
}
