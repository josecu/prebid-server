package contextualapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/golang/glog"
	"github.com/prebid/openrtb/v17/openrtb2"
	"github.com/prebid/prebid-server/hooks/hookstage"
	"github.com/prebid/prebid-server/modules/moduledeps"
)

// This could go in a ArcSpan module YAML config if/when modules support YAML config files
const arcspanEndpoint = "http://pbs{{.SILO}}.p7cloud.net/ctx"

var endpoint string

func Builder(config json.RawMessage, deps moduledeps.ModuleDeps) (interface{}, error) {
	endpoint = arcspanEndpoint
	if config != nil {
		var arcGlobalConfig ArcGlobalConfig
		if err := json.Unmarshal(config, &arcGlobalConfig); err != nil {
			glog.Error("ARCSPAN:: Error reading from global config (" + err.Error() + ")")
		}
		if arcGlobalConfig.Endpoint != "" {
			endpoint = arcGlobalConfig.Endpoint
		}
	}
	return Module{}, nil
}

type Module struct{}

func (m Module) HandleProcessedAuctionHook(
	_ context.Context,
	miCtx hookstage.ModuleInvocationContext,
	payload hookstage.ProcessedAuctionRequestPayload,
) (hookstage.HookResult[hookstage.ProcessedAuctionRequestPayload], error) {
	glog.Info("ARCSPAN:: Processed Auction Hook | Start")
	result := hookstage.HookResult[hookstage.ProcessedAuctionRequestPayload]{}
	var arcAccount ArcAccount
	if err := json.Unmarshal(miCtx.AccountConfig, &arcAccount); err != nil {
		return result, errors.New("ARCSPAN:: Processed Auction Hook | Error reading account information (" + err.Error() + ")")
	}
	if arcAccount.Silo == "" {
		return result, errors.New("ARCSPAN:: Processed Auction Hook | Invalid silo ID provided")
	}
	glog.Infof("ARCSPAN:: Processed Auction Hook | Silo %s", arcAccount.Silo)
	site, err := fetchContextual(payload, arcAccount.Silo)
	if err != nil {
		return result, err
	}
	changeSet := hookstage.ChangeSet[hookstage.ProcessedAuctionRequestPayload]{}
	changeSet.AddMutation(func(payload hookstage.ProcessedAuctionRequestPayload) (hookstage.ProcessedAuctionRequestPayload, error) {
		payload.BidRequest.Site = site
		return payload, nil
	}, hookstage.MutationUpdate, "bidrequest", "site")
	result.ChangeSet = changeSet
	glog.Info("ARCSPAN:: Processed Auction Hook | End")

	return result, nil
}

func fetchContextual(payload hookstage.ProcessedAuctionRequestPayload, silo string) (*openrtb2.Site, error) {
	var hasSite bool = payload.BidRequest.Site != nil
	if !hasSite {
		return nil, errors.New("ARCSPAN:: Processed Auction Hook | No site oject included in request. Unable to add contextual data")
	}
	var hasPage bool = payload.BidRequest.Site.Page != ""
	if !hasPage {
		return nil, errors.New("ARCSPAN:: Processed Auction Hook | Site object does not contain a page url. Unable to add contextual data")
	}
	var url string = strings.Replace(endpoint, "{{.SILO}}", silo, 1) + "?format=json&uri=" + payload.BidRequest.Site.Page
	glog.Info("ARCSPAN:: Fetching contextual information from " + url)
	resp, err := http.Get(url)
	if err != nil {
		return nil, errors.New("ARCSPAN:: Processed Auction Hook | Encountered network error fetching contextual information")
	}
	defer resp.Body.Close()
	arcObject, err := processResponse(resp)
	if err != nil {
		return nil, err
	}
	site := augmentPayload(*arcObject, payload)
	return &site, nil
}

func processResponse(response *http.Response) (*ArcObject, error) {
	if response.StatusCode != http.StatusOK {
		return nil, errors.New("ARCSPAN:: Processed Auction Hook | Received unknown status code (" + fmt.Sprint(response.StatusCode) + ")")
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, errors.New("ARCSPAN:: Processed Auction Hook | Error downloading response (" + err.Error() + ")")
	}
	glog.Infof("ARCSPAN:: Processed Auction Hook | Cloudfront Response: %s\n", body)
	var arcObject ArcObject
	if err := json.Unmarshal(body, &arcObject); err != nil {
		return nil, errors.New("ARCSPAN:: Processed Auction Hook | Error parsing response (" + err.Error() + ")")
	}
	return &arcObject, nil
}

func augmentPayload(arcObject ArcObject, payload hookstage.ProcessedAuctionRequestPayload) openrtb2.Site {
	var hasContent bool = payload.BidRequest.Site.Content != nil
	var hasData bool = hasContent && payload.BidRequest.Site.Content.Data != nil

	var v1 []string
	var v1s []string
	var v2 []string

	if arcObject.Codes != nil {
		v1 = arcObject.Codes.Text
		v1 = append(v1, arcObject.Codes.Images...)
	}

	if arcObject.Raw != nil {
		v1s = arcObject.Raw.Text
		v1s = append(v1s, arcObject.Raw.Images...)
	}

	if arcObject.NewCodes != nil {
		v2 = arcObject.NewCodes.Text
		v2 = append(v2, arcObject.NewCodes.Images...)
	}

	var segments []openrtb2.Segment
	for _, segmentId := range v2 {
		segments = append(segments, openrtb2.Segment{ID: segmentId})
	}

	ext := json.RawMessage(`{ "segtax": 6 }`)

	var data []openrtb2.Data
	if hasData {
		data = payload.BidRequest.Site.Content.Data
	}

	arcspanData := openrtb2.Data{Name: "arcspan", Segment: segments, Ext: ext}
	data = append(data, arcspanData)

	var content openrtb2.Content
	if hasContent {
		content = *payload.BidRequest.Site.Content
		content.Data = data
	} else {
		content = openrtb2.Content{Data: data}
	}

	site := payload.BidRequest.Site
	site.Name = "arcspan"
	site.Cat = v1
	site.SectionCat = v1
	site.PageCat = v1
	site.Keywords = strings.Join(v1s, ",")
	site.Content = &content

	return *site
}

type ArcCodes struct {
	Images []string `json:"images"`
	Text   []string `json:"text"`
}

type ArcObject struct {
	Raw      *ArcCodes `json:"raw"`
	Codes    *ArcCodes `json:"codes"`
	NewCodes *ArcCodes `json:"newCodes"`
}

type ArcAccount struct {
	Silo string `json:"silo"`
}

type ArcGlobalConfig struct {
	Endpoint string `json:"endpoint"`
}
