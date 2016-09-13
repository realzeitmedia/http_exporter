package main

import (
	"time"
)

type Config struct {
	Targets    map[string]Target `yaml:"targets"`
	Timeout    time.Duration     `yaml:"timeout"` // default defaultTimeout
	SpiderTime time.Duration     `yaml:"spider"`  // default defaultSpiderTime
}

type Target struct {
	URL        string        `yaml:"url"`
	Method     string        `yaml:"method"`     // GET/HEAD/POST/...
	Timeout    time.Duration `yaml:"timeout"`    // default main Timeout
	SpiderTime time.Duration `yaml:"spidertime"` // default main SpiderTime
}
