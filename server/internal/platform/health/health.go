package health

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
)

type Probe func(context.Context) error

type Checker struct {
	timeout time.Duration
	probes  map[string]Probe
}

type dependencyStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type response struct {
	Status       string             `json:"status"`
	Dependencies []dependencyStatus `json:"dependencies,omitempty"`
}

func NewChecker(timeout time.Duration, probes map[string]Probe) *Checker {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	copyProbes := make(map[string]Probe, len(probes))
	for name, probe := range probes {
		copyProbes[name] = probe
	}
	return &Checker{timeout: timeout, probes: copyProbes}
}

func Live(c *gin.Context) {
	c.JSON(http.StatusOK, response{Status: "ok"})
}

func (checker *Checker) Ready(c *gin.Context) {
	statuses, err := checker.Check(c.Request.Context())
	if err != nil {
		_ = c.Error(err).SetType(gin.ErrorTypePrivate)
		c.JSON(http.StatusServiceUnavailable, response{Status: "unavailable", Dependencies: statuses})
		return
	}
	c.JSON(http.StatusOK, response{Status: "ok", Dependencies: statuses})
}

func (checker *Checker) Check(ctx context.Context) ([]dependencyStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, checker.timeout)
	defer cancel()

	type result struct {
		name string
		err  error
	}
	results := make(chan result, len(checker.probes))
	for name, probe := range checker.probes {
		go func(name string, probe Probe) {
			if probe == nil {
				results <- result{name: name, err: errors.New("probe is not configured")}
				return
			}
			results <- result{name: name, err: probe(ctx)}
		}(name, probe)
	}

	statuses := make([]dependencyStatus, 0, len(checker.probes))
	var failures []error
	completed := make(map[string]bool, len(checker.probes))
	for range checker.probes {
		var result result
		select {
		case result = <-results:
			completed[result.name] = true
		case <-ctx.Done():
			for name := range checker.probes {
				if !completed[name] {
					statuses = append(statuses, dependencyStatus{Name: name, Status: "unavailable"})
					failures = append(failures, errors.New(name+": "+ctx.Err().Error()))
				}
			}
			sort.Slice(statuses, func(i, j int) bool { return statuses[i].Name < statuses[j].Name })
			return statuses, errors.Join(failures...)
		}
		status := "ok"
		if result.err != nil {
			status = "unavailable"
			failures = append(failures, errors.New(result.name+": "+result.err.Error()))
		}
		statuses = append(statuses, dependencyStatus{Name: result.name, Status: status})
	}
	sort.Slice(statuses, func(i, j int) bool { return statuses[i].Name < statuses[j].Name })
	return statuses, errors.Join(failures...)
}
