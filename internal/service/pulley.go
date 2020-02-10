package service

import (
	"sync"

	"github.com/knl/pulley/internal/metrics"
)

type Pulley struct {
	Updates chan interface{}
	Metrics metrics.Publisher
	Token   []byte
	WG      sync.WaitGroup
}
