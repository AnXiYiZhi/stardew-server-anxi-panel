package registry

import "sort"

type Registry struct {
	drivers map[string]GameDriver
}

func New() *Registry {
	return &Registry{drivers: make(map[string]GameDriver)}
}

func (r *Registry) Register(driver GameDriver) error {
	if driver == nil || driver.ID() == "" {
		return ErrInvalidDriver
	}
	if _, exists := r.drivers[driver.ID()]; exists {
		return ErrDriverAlreadyRegistered
	}
	r.drivers[driver.ID()] = driver
	return nil
}

func (r *Registry) Get(driverID string) (GameDriver, error) {
	if driverID == "" {
		return nil, ErrDriverNotFound
	}
	driver, ok := r.drivers[driverID]
	if !ok {
		return nil, ErrDriverNotFound
	}
	return driver, nil
}

func (r *Registry) List() []DriverInfo {
	infos := make([]DriverInfo, 0, len(r.drivers))
	for _, driver := range r.drivers {
		infos = append(infos, DriverInfo{ID: driver.ID(), Name: driver.Name()})
	}
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].ID < infos[j].ID
	})
	return infos
}
