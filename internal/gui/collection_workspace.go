package gui

type collectionWorkspaceState struct {
	selectedID string
}

// InitializeCollectionWorkspace seeds an empty store once at desktop startup.
func (a *App) InitializeCollectionWorkspace(defaultName string) error {
	settings := a.Svc.Settings.Get()
	if settings.CollectionWorkspaceInitialized {
		return nil
	}
	if len(a.Svc.Collections.List()) == 0 {
		if _, err := a.Svc.Collections.Create(defaultName); err != nil {
			return err
		}
	}
	settings.CollectionWorkspaceInitialized = true
	return a.Svc.Settings.Save(settings)
}

func (a *App) workspaceCollectionID() string {
	if a.active >= 0 && a.active < len(a.tabs) {
		if id := a.Active().CollectionID; id != "" {
			if _, ok := a.Svc.Collections.Get(id); ok {
				return id
			}
		}
	}
	selectedID := a.collectionWorkspace.selectedID
	if selectedID == "" {
		selectedID = a.Svc.Settings.Get().WorkspaceCollectionID
	}
	if selectedID != "" {
		if _, ok := a.Svc.Collections.Get(selectedID); ok {
			return selectedID
		}
	}
	var firstID, firstName string
	for _, col := range a.Svc.Collections.List() {
		if col.ID == "" {
			continue
		}
		if firstID == "" || col.Name < firstName || (col.Name == firstName && col.ID < firstID) {
			firstID, firstName = col.ID, col.Name
		}
	}
	return firstID
}

// selectWorkspaceCollection remembers a valid explicit selection. The active
// tab's collection still takes precedence until the caller changes tabs.
func (a *App) selectWorkspaceCollection(id string) {
	if id == "" {
		return
	}
	if _, ok := a.Svc.Collections.Get(id); !ok {
		return
	}
	a.collectionWorkspace.selectedID = id
	settings := a.Svc.Settings.Get()
	if settings.WorkspaceCollectionID != id {
		settings.WorkspaceCollectionID = id
		_ = a.Svc.Settings.Save(settings)
	}
}
