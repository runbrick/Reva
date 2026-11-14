package main

import (
	"github.com/runbrick/reva/internal/service"
	"github.com/runbrick/reva/internal/service/fake"
	"github.com/runbrick/reva/internal/service/request"
	"github.com/runbrick/reva/internal/service/store"
	"github.com/runbrick/reva/internal/storage"
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
	svc.Requests = request.New()
	return svc, nil
}
