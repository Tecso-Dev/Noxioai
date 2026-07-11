package main

import "context"

// Agent contract — SPEC §4. Dispatch is a switch in main, added with the
// first real agent (ORACLE, Phase 2). A registry waits until 5+ agents exist.

type Task struct {
	Agent string // oracle | atlas
	Input string // e.g. "real estate agencies in Warsaw" or a lead id
}

type Result struct {
	Output string
}

type Agent interface {
	Name() string
	Run(ctx context.Context, task Task) (Result, error)
}
