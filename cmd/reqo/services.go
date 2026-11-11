package main

import (
	"github.com/runbrick/reqo/internal/service"
	"github.com/runbrick/reqo/internal/service/fake"
	"github.com/runbrick/reqo/internal/service/request"
)

// services builds the service set the app runs on. Each service PR replaces
// its fake.New() field with the disk- or network-backed implementation,
// and touches only its own line.
func services() (service.Services, error) {
	svc := fake.New()
	svc.Requests = request.New()
	return svc, nil
}
