package about

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const sentinel = "RAILS_KIT_ABOUT_JSON="

const runtimeScript = `
require "json"
require "rack"
payload = {
  rails: Rails.version,
  ruby: RUBY_DESCRIPTION,
  rubygems: Gem::VERSION,
  rack: Rack.release,
  bundler: Bundler::VERSION,
  environment: Rails.env.to_s,
  database_adapter: (ActiveRecord::Base.connection_db_config.adapter rescue nil)
}
puts "` + sentinel + `" + JSON.generate(payload)
`

type RuntimeInfo struct {
	Rails           string `json:"rails"`
	Ruby            string `json:"ruby"`
	RubyGems        string `json:"rubygems"`
	Rack            string `json:"rack"`
	Bundler         string `json:"bundler"`
	Environment     string `json:"environment"`
	DatabaseAdapter string `json:"database_adapter"`
}

type Runner struct {
	Bundle string
}

func (runner Runner) Inspect(ctx context.Context, root string) (RuntimeInfo, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
	}
	bundle := runner.Bundle
	if bundle == "" {
		bundle = "bundle"
	}
	command := exec.CommandContext(ctx, bundle, "exec", "rails", "runner", runtimeScript)
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return RuntimeInfo{}, fmt.Errorf("timed out")
		}
		return RuntimeInfo{}, fmt.Errorf("Rails runner exited: %w", err)
	}
	for _, line := range strings.Split(string(output), "\n") {
		if !strings.HasPrefix(line, sentinel) {
			continue
		}
		var info RuntimeInfo
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, sentinel)), &info); err != nil {
			return RuntimeInfo{}, fmt.Errorf("invalid runtime response: %w", err)
		}
		return info, nil
	}
	return RuntimeInfo{}, fmt.Errorf("runtime response marker not found")
}

func Enrich(report Report, info RuntimeInfo) Report {
	report.Source = "runtime"
	if info.Rails != "" {
		report.Versions.Rails = info.Rails
	}
	if info.Ruby != "" {
		report.Versions.Ruby = info.Ruby
	}
	if info.RubyGems != "" {
		report.Versions.RubyGems = info.RubyGems
	}
	if info.Rack != "" {
		report.Versions.Rack = info.Rack
	}
	if info.Bundler != "" {
		report.Versions.Bundler = info.Bundler
	}
	if info.Environment != "" {
		report.Environment = info.Environment
	}
	if info.DatabaseAdapter != "" {
		report.Database.Adapters = []string{info.DatabaseAdapter}
	}
	return report
}
