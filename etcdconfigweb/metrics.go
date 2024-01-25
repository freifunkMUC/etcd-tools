package main

import (
	"encoding/json"
	"net/http"
	"log"
	"context"
)

type RequestTracker interface {
	RequestSuccessful()
	RequestFailed()
}


type MetricEvent uint

const (
	REQUEST_SUCCESSFUL MetricEvent = iota
	REQUEST_FAILED
)

type NodeCounter interface {
	NodeCount(ctx context.Context) (uint64, error)
}

type Metrics struct {
	RequestsFailed uint64
	RequestsSuccessful uint64

	events chan MetricEvent
	counter NodeCounter
}

func NewMetrics(counter NodeCounter) *Metrics {
	metrics := &Metrics {
		events: make(chan MetricEvent, 64),
		counter: counter,
	}
	go metrics.EventLoop()

	return metrics
}

func (m *Metrics) RequestSuccessful() {
	m.events <- REQUEST_SUCCESSFUL
}

func (m *Metrics) RequestFailed() {
	m.events <- REQUEST_FAILED
}

func (m *Metrics) EventLoop() {
	for event := range m.events {
		switch(event) {
			case REQUEST_SUCCESSFUL:
				m.RequestsSuccessful += 1
			case REQUEST_FAILED:
				m.RequestsFailed += 1
			default:
				panic("Undefined/unhandled event")
		}
	}
}

type MetricResponse struct {
	RequestsFailed uint64 `json:"requestsFailed"`
	RequestsSuccessful uint64 `json:"requestsSuccessful"`
	NodesConfigured uint64 `json:"nodesConfigured"`
}

func (m *Metrics) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	count, err := m.counter.NodeCount(context.Background()) // TODO use proper context
	if err != nil {
		log.Println("Error trying to get the node count:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(&MetricResponse{
		RequestsFailed: m.RequestsFailed,
		RequestsSuccessful: m.RequestsSuccessful,
		NodesConfigured: count,
	}); err != nil {
		log.Println("Error while serving status request", err)
	}
}
