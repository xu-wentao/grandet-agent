// Package testkit provides reusable in-memory doubles for application tests.
package testkit

import (
	"time"

	"github.com/xu-wentao/grandet-agent/internal/application"
	"github.com/xu-wentao/grandet-agent/internal/domain"
)

type FixedClock struct{ Time time.Time }

func (c FixedClock) Now() time.Time { return c.Time }

type FixedIDGenerator struct{ ID string }

func (g FixedIDGenerator) New() string { return g.ID }

type WorkspaceFilesystem struct {
	MkdirAllFunc  func(string) error
	WriteFileFunc func(path, content string, force bool) (bool, error)
}

func (f WorkspaceFilesystem) MkdirAll(path string) error {
	if f.MkdirAllFunc == nil {
		return nil
	}
	return f.MkdirAllFunc(path)
}

func (f WorkspaceFilesystem) WriteFile(path, content string, force bool) (bool, error) {
	if f.WriteFileFunc == nil {
		return false, nil
	}
	return f.WriteFileFunc(path, content, force)
}

type WorkspaceDatabase struct {
	MigrateFunc        func(string) error
	RecordVersionsFunc func(path string, versions map[string]string) error
}

func (d WorkspaceDatabase) Migrate(path string) error {
	if d.MigrateFunc == nil {
		return nil
	}
	return d.MigrateFunc(path)
}

func (d WorkspaceDatabase) RecordVersions(path string, versions map[string]string) error {
	if d.RecordVersionsFunc == nil {
		return nil
	}
	return d.RecordVersionsFunc(path, versions)
}

var (
	_ domain.Clock                    = FixedClock{}
	_ domain.IDGenerator              = FixedIDGenerator{}
	_ application.WorkspaceFilesystem = WorkspaceFilesystem{}
	_ application.WorkspaceDatabase   = WorkspaceDatabase{}
)
