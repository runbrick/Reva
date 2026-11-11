package main

import (
	"github.com/runbrick/reqo/internal/service"
	"github.com/runbrick/reqo/internal/service/fake"
	"github.com/runbrick/reqo/internal/service/store"
	"github.com/runbrick/reqo/internal/storage"
)

// services builds the service set the app runs on. Each service PR replaces
// its fake.New() field with the disk- or network-backed implementation,
// and touches only its own line.
func services() (service.Services, error) {
	svc := fake.New()
	dir, err := storage.Dir()
	if err != nil {
		return service.Services{}, err
	}
	disk, err := store.Open(dir)
	if err != nil {
		return service.Services{}, err
	}
	svc.History = disk.History()
	svc.Collections = disk.Collections()
	svc.Environments = disk.Environments()
	svc.Settings = disk.Settings()
	return svc, nil
}
