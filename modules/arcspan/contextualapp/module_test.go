package contextualapp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prebid/openrtb/v17/openrtb2"
	"github.com/prebid/prebid-server/hooks/hookexecution"
	"github.com/prebid/prebid-server/hooks/hookstage"
	"github.com/prebid/prebid-server/modules/moduledeps"
	"github.com/stretchr/testify/assert"
)

var invalidConfig = json.RawMessage(`{

}
`)

var validConfig = json.RawMessage(`
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
			description:        "Invalid config",
			config:             invalidConfig,
			bidRequest:         &openrtb2.BidRequest{},
			expectedBidRequest: &openrtb2.BidRequest{},
			expectedHookResult: hookstage.HookResult[hookstage.ProcessedAuctionRequestPayload]{},
			expectedError:      errors.New("ARCSPAN:: Processed Auction Hook | Invalid silo ID provided"),
		},
		{
			description:        "Bid Request with no Site object",
			config:             validConfig,
			bidRequest:         &openrtb2.BidRequest{},
			expectedBidRequest: &openrtb2.BidRequest{},
			expectedHookResult: hookstage.HookResult[hookstage.ProcessedAuctionRequestPayload]{},
			expectedError:      errors.New("ARCSPAN:: Processed Auction Hook | No site oject included in request. Unable to add contextual data"),
		},
		{
			description: "Bid Request with no page URL",
			config:      validConfig,
			bidRequest: &openrtb2.BidRequest{
				Site: &openrtb2.Site{},
			},
			expectedBidRequest: &openrtb2.BidRequest{
				Site: &openrtb2.Site{},
			},
			expectedHookResult: hookstage.HookResult[hookstage.ProcessedAuctionRequestPayload]{},
			expectedError:      errors.New("ARCSPAN:: Processed Auction Hook | Site object does not contain a page url. Unable to add contextual data"),
		},
		{
			description: "Valid Bid Request",
			config:      validConfig,
			bidRequest: &openrtb2.BidRequest{
				Site: &openrtb2.Site{
					Page: "https://sportsnaut.com/dallas-cowboys-vs-tampa-bay-buccaneers-preview/",
				},
			},
			expectedBidRequest: &openrtb2.BidRequest{
				Site: &openrtb2.Site{
					Page:       "https://sportsnaut.com/dallas-cowboys-vs-tampa-bay-buccaneers-preview/",
					Name:       "arcspan",
					Cat:        []string{"IAB17", "IAB17-44"},
					SectionCat: []string{"IAB17", "IAB17-44"},
					PageCat:    []string{"IAB17", "IAB17-44"},
					Keywords:   "Sports>Soccer,Sports>Football",
					Content: &openrtb2.Content{
						Data: []openrtb2.Data{
							{
								Name: "arcspan",
								Segment: []openrtb2.Segment{
									{ID: "483"},
									{ID: "533"},
								},
								Ext: json.RawMessage(`{ "segtax": 6 }`),
							},
						},
					},
				},
			},
			expectedHookResult: hookstage.HookResult[hookstage.ProcessedAuctionRequestPayload]{},
			expectedError:      nil,
		},
	}

	for _, test := range testCases {
		t.Run(test.description, func(t *testing.T) {
			payload := hookstage.ProcessedAuctionRequestPayload{BidRequest: test.bidRequest}

			stubHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				b := []byte("aspan.setIAB({\"raw\": {\"text\": [\"Sports>Soccer\", \"Sports>Football\"]}, \"codes\": {\"text\": [\"IAB17\", \"IAB17-44\"]}, \"newcodes\": {\"text\": [\"483\", \"533\"]}})")
				w.Write(b)
			})
			stubServer := httptest.NewServer(stubHandler)
			defer stubServer.Close()

			config := json.RawMessage("{\"enabled\":true,\"endpoint\":\"" + stubServer.URL + "\"}")

			result, err := Builder(config, moduledeps.ModuleDeps{})
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
			assert.Equal(t, test.expectedBidRequest, payload.BidRequest, "Invalid BidRequest after executing ProcessedAuctionHook.")

			// reset ChangeSet not to break hookResult assertion, we validated ChangeSet separately
			hookResult.ChangeSet = hookstage.ChangeSet[hookstage.ProcessedAuctionRequestPayload]{}
			assert.Equal(t, test.expectedHookResult, hookResult, "Invalid hook execution result.")
		})
	}
}
