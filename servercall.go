package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// dont forgot to defer resp.Body.Close()
func (g *gui) servercall(method string, path string, body io.Reader, header map[string][]string) (*http.Response, error) {
	invoke, err := url.Parse(path)
	if err != nil {
		return nil, err
	}
	invoke.Scheme = "http"
	invoke.Host = g.server

	//fmt.Println(invoke.String()) //log
	req, err := http.NewRequest(method, invoke.String(), body)
	if err != nil {
		return nil, err
	}
	for k, v := range header {
		for _, vv := range v {
			req.Header.Add(k, vv)
		}
	}

	resp, err := g.h.Do(req)

	if err != nil {
		return resp, err
	}

	if resp.StatusCode == 422 || resp.StatusCode == 415 {
		defer resp.Body.Close()
		content, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("422: %v", string(content))
	}

	return resp, err
}

type queueInvoked struct {
	Pending    int `json:"pending"`
	InProgress int `json:"in_progress"`
}
type statusInvoked struct {
	Queue queueInvoked `json:"queue"`
}

func parseStatus(r io.Reader) (statusInvoked, error) {
	d, err := io.ReadAll(r)
	if err != nil {
		return statusInvoked{}, fmt.Errorf("failed to unmarshal status: %w", err)
	}

	var av statusInvoked
	err = json.Unmarshal(d, &av)
	if err != nil {
		return statusInvoked{}, fmt.Errorf("failed to unmarshal status: %w", err)
	}

	return av, nil
}

type queueClear struct {
	Deleted int `json:"deleted"`
}

func parseClear(r io.Reader) (queueClear, error) {
	d, err := io.ReadAll(r)
	if err != nil {
		return queueClear{}, fmt.Errorf("failed to unmarshal clear: %w", err)
	}

	var av queueClear
	err = json.Unmarshal(d, &av)
	if err != nil {
		return queueClear{}, fmt.Errorf("failed to unmarshal clear: %w", err)
	}

	return av, nil
}
