// Package testkit contains small configurable doubles shared by package tests.
package testkit

import (
	"context"
	"time"
)

type Clock struct{ Time time.Time }

func (c Clock) Now() time.Time { return c.Time }

type IDGenerator struct{ ID string }

func (g IDGenerator) New() string { return g.ID }

type Provider[Request, Response any] struct {
	Call func(context.Context, Request) (Response, error)
}

func (p Provider[Request, Response]) Invoke(ctx context.Context, request Request) (Response, error) {
	return p.Call(ctx, request)
}

type Repository[ID comparable, Value any] struct {
	Get func(context.Context, ID) (Value, error)
	Put func(context.Context, Value) error
}

type Validator[Value any] func(Value) error

func (v Validator[Value]) Validate(value Value) error { return v(value) }
