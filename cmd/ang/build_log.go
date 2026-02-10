package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/strogmv/ang/compiler/generator"
)

type buildEvent struct {
	Timestamp      string   `json:"ts"`
	Stage          string   `json:"stage"`
	Target         string   `json:"target,omitempty"`
	Step           string   `json:"step,omitempty"`
	Status         string   `json:"status"`
	DurationMS     int64    `json:"duration_ms,omitempty"`
	MissingCaps    []string `json:"missing_caps,omitempty"`
	FilesGenerated int      `json:"files_generated,omitempty"`
	Warnings       int      `json:"warnings,omitempty"`
	Error          string   `json:"error,omitempty"`
	Message        string   `json:"message,omitempty"`
}

func emitBuildEvent(ev buildEvent) {
	b, _ := json.Marshal(ev)
	fmt.Fprintln(os.Stdout, string(b))
}

func mapStepEvent(ev generator.StepEvent) buildEvent {
	missing := make([]string, 0, len(ev.MissingCaps))
	for _, c := range ev.MissingCaps {
		missing = append(missing, string(c))
	}
	return buildEvent{
		Timestamp:      time.Now().UTC().Format(time.RFC3339Nano),
		Stage:          ev.Stage,
		Target:         ev.Target,
		Step:           ev.Step,
		Status:         ev.Status,
		DurationMS:     ev.DurationMS,
		MissingCaps:    missing,
		FilesGenerated: ev.FilesGenerated,
		Warnings:       ev.Warnings,
		Error:          ev.Error,
	}
}
