package tplink_test

import (
	"testing"

	"github.com/t0mer/SwitchDeck/internal/switchclient"
	"github.com/t0mer/SwitchDeck/internal/switchclient/tplink"
)

func TestTPLinkImplementsClient(t *testing.T) {
	var _ switchclient.Client = (*tplink.TPLink)(nil)
}
