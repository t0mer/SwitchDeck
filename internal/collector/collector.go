package collector

import (
	"context"
	"fmt"

	"github.com/t0mer/SwitchDeck/internal/models"
	"github.com/t0mer/SwitchDeck/internal/switchclient"
)

// Collector gathers all available data from a single switch.
type Collector struct {
	client switchclient.Client
}

// New creates a Collector backed by the given client.
func New(client switchclient.Client) *Collector {
	return &Collector{client: client}
}

// Collect runs a full data collection pass and returns a snapshot.
func (c *Collector) Collect(ctx context.Context) (*models.SwitchSnapshot, error) {
	snap, err := c.client.GetSnapshot(ctx)
	if err != nil {
		return nil, fmt.Errorf("collecting snapshot: %w", err)
	}
	return snap, nil
}
