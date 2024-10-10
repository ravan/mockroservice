package util

import (
	"log/slog"
	"regexp"
	"time"
)

type Delay struct {
	Enabled        bool
	BeforeDuration time.Duration
	AfterDuration  time.Duration
}

func (d *Delay) ApplyBefore(service, callTarget string) {
	if d.Enabled && d.BeforeDuration.Milliseconds() > 0 {
		slog.Debug("latency before", "service", service, "ms", d.BeforeDuration.Milliseconds(), "target", callTarget)
		time.Sleep(d.BeforeDuration)
	}
}

func (d *Delay) ApplyAfter(service, callTarget string) {
	if d.Enabled && d.AfterDuration.Milliseconds() > 0 {
		slog.Debug("latency after", "service", service, "ms", d.AfterDuration.Milliseconds(), "target", callTarget)
		time.Sleep(d.AfterDuration)
	}
}

const (
	aroundPattern = "^<([0-9a-z]*)>$"
	beforePattern = "^([0-9a-z]*)[<]?"
	afterPattern  = "[>]([0-9a-z]*)$"
	bothPattern   = "([0-9a-z]*)[<][>]([0-9a-z]*)"
)

var (
	aroundRegexp = regexp.MustCompile(aroundPattern)
	beforeRegexp = regexp.MustCompile(beforePattern)
	afterRegexp  = regexp.MustCompile(afterPattern)
	bothRegexp   = regexp.MustCompile(bothPattern)
)

func ParseDelay(delay string) *Delay {

	before := "0ms"
	after := "0ms"
	if delay != "" {
		if aroundMatch := aroundRegexp.FindStringSubmatch(delay); aroundMatch != nil {
			before = aroundMatch[1]
			after = aroundMatch[1]
		} else if bothMatch := bothRegexp.FindStringSubmatch(delay); bothMatch != nil {
			before = bothMatch[1]
			after = bothMatch[2]
		} else if beforeMatch := beforeRegexp.FindStringSubmatch(delay); beforeMatch != nil {
			before = beforeMatch[1]
		} else if afterMatch := afterRegexp.FindStringSubmatch(delay); afterMatch != nil {
			after = afterMatch[1]
		}
	}

	disabled := false
	beforeDuration, err := time.ParseDuration(before)
	if err != nil {
		disabled = true
		slog.Error("failed to parse duration", "delay", delay, "duration", before)
	}
	afterDuration, err := time.ParseDuration(after)
	if err != nil {
		disabled = true
		slog.Error("failed to parse duration", "delay", delay, "duration", after)
	}
	return &Delay{
		Enabled:        !disabled,
		BeforeDuration: beforeDuration,
		AfterDuration:  afterDuration,
	}
}
