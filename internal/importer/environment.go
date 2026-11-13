package importer

import (
	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/storage"
)

func (c *conversion) environment(m object) *model.Environment {
	if str(m, "_postman_variable_scope", "$") != "environment" {
		fail("unsupported_format", "$._postman_variable_scope")
	}
	e := &model.Environment{Version: model.SchemaVersion, ID: storage.NewID(), Name: name(m, "$")}
	e.Variables = c.postmanRows(m["values"], "$.values")
	c.metadata(m, "$", "name", "values", "_postman_variable_scope")
	return e
}
