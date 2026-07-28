// Package testkit provides reusable in-memory doubles for application tests.
package testkit

import (
	"context"
	"time"

	"github.com/xu-wentao/grandet-agent/internal/application"
	"github.com/xu-wentao/grandet-agent/internal/domain"
)

type FixedClock struct{ Time time.Time }

func (c FixedClock) Now() time.Time { return c.Time }

type FixedIDGenerator struct{ ID string }

func (g FixedIDGenerator) New() string { return g.ID }

// FakeProvider records Execute calls and delegates results to ExecuteFunc.
type FakeProvider[Request, Response any] struct {
	ExecuteFunc func(context.Context, Request) (Response, error)
	Requests    []Request
}

func (f *FakeProvider[Request, Response]) Execute(ctx context.Context, request Request) (Response, error) {
	f.Requests = append(f.Requests, request)
	if f.ExecuteFunc != nil {
		return f.ExecuteFunc(ctx, request)
	}
	var zero Response
	return zero, nil
}

// FakeRepository records Get calls and delegates results to GetFunc.
type FakeRepository[Key, Value any] struct {
	GetFunc func(context.Context, Key) (Value, error)
	Keys    []Key
}

func (f *FakeRepository[Key, Value]) Get(ctx context.Context, key Key) (Value, error) {
	f.Keys = append(f.Keys, key)
	if f.GetFunc != nil {
		return f.GetFunc(ctx, key)
	}
	var zero Value
	return zero, nil
}

// FakeValidator records Validate calls and delegates results to ValidateFunc.
type FakeValidator[Input, Result any] struct {
	ValidateFunc func(context.Context, Input) (Result, error)
	Inputs       []Input
}

func (f *FakeValidator[Input, Result]) Validate(ctx context.Context, input Input) (Result, error) {
	f.Inputs = append(f.Inputs, input)
	if f.ValidateFunc != nil {
		return f.ValidateFunc(ctx, input)
	}
	var zero Result
	return zero, nil
}

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
