package contextualapp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/prebid/openrtb/v17/openrtb2"
	"github.com/prebid/prebid-server/hooks/hookexecution"
	"github.com/prebid/prebid-server/hooks/hookstage"
	"github.com/prebid/prebid-server/modules/moduledeps"
	"github.com/stretchr/testify/assert"
)

var testConfig = json.RawMessage(`
{
	"silo":"21"
}
`)

func TestHandleProcessedAuctionHook(t *testing.T) {
	testCases := []struct {
		description        string
		config             json.RawMessage
		bidRequest         *openrtb2.BidRequest
		expectedBidRequest *openrtb2.BidRequest
		expectedHookResult hookstage.HookResult[hookstage.ProcessedAuctionRequestPayload]
		expectedError      error
	}{
		{
			description:        "Bid Request with no Site object.",
			config:             testConfig,
			bidRequest:         &openrtb2.BidRequest{},
			expectedBidRequest: &openrtb2.BidRequest{},
			expectedHookResult: hookstage.HookResult[hookstage.ProcessedAuctionRequestPayload]{},
			expectedError:      errors.New("ARCSPAN:: Processed Auction Hook | No site oject included in request. Unable to add contextual data"),
		},
	}

	for _, test := range testCases {
		t.Run(test.description, func(t *testing.T) {
			payload := hookstage.ProcessedAuctionRequestPayload{BidRequest: test.bidRequest}

			result, err := Builder(nil, moduledeps.ModuleDeps{})
			assert.NoError(t, err, "Failed to build module.")

			module, ok := result.(Module)
			assert.True(t, ok, "Failed to cast module type.")

			hookResult, err := module.HandleProcessedAuctionHook(
				context.Background(),
				hookstage.ModuleInvocationContext{
					AccountConfig: test.config,
					Endpoint:      hookexecution.EndpointAuction,
					ModuleContext: map[string]interface{}{},
				},
				payload,
			)
			assert.Equal(t, test.expectedError, err, "Invalid hook execution error.")

			// test mutations separately
			for _, mut := range hookResult.ChangeSet.Mutations() {
				_, err := mut.Apply(payload)
				assert.NoError(t, err)
			}
			assert.Equal(t, test.expectedBidRequest, payload.BidRequest, "Invalid BidRequest after executing BidderRequestHook.")

			// reset ChangeSet not to break hookResult assertion, we validated ChangeSet separately
			hookResult.ChangeSet = hookstage.ChangeSet[hookstage.ProcessedAuctionRequestPayload]{}
			assert.Equal(t, test.expectedHookResult, hookResult, "Invalid hook execution result.")
		})
	}
}
