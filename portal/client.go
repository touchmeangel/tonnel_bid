package portal

import (
	"autobid/tlsclient"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"time"
)

type PortalAPI struct {
	conn *tlsclient.TLSClient
	opt  *Options
}

type Options struct {
	Proxies      []*url.URL
	FloodRetries uint32
}

func New(opt *Options) (*PortalAPI, error) {
	if opt == nil {
		opt = &Options{}
	}

	c, err := tlsclient.New("portals-market.com", false, false, opt.Proxies)
	if err != nil {
		return nil, err
	}
	return &PortalAPI{conn: c, opt: opt}, nil
}

const FLOOD_WAIT time.Duration = time.Second * 5

var DEFAULT_HEADERS = map[string]string{
	"accept":     "application/json, text/plain, */*",
	"user-agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36",
	"connection": "keep-alive",
}

type FloorPrices struct {
	Collections map[string]interface{}           `json:"collections"`
	FloorPrices map[string]CollectionFloorPrices `json:"floor_prices"`
}

type CollectionFloorPrices struct {
	Backdrops map[string]string `json:"backdrops"`
	Models    map[string]string `json:"models"`
	Symbols   map[string]string `json:"symbols"`
}

func (api *PortalAPI) GetFloor(giftName string) (*FloorPrices, error) {
	url, err := tlsclient.SafeURL("https://portals-market.com/api/collections/filters", map[string]string{
		"short_names": giftName,
	})
	if err != nil {
		return nil, err
	}

	headers := make(map[string]string, len(DEFAULT_HEADERS))
	for k, v := range DEFAULT_HEADERS {
		headers[k] = v
	}

	var resp *tlsclient.RequestResponse
	i := uint32(0)
	t := FLOOD_WAIT
	for {
		resp, err = api.conn.Request("GET", url, nil, headers, 3)
		if err != nil {
			return nil, err
		}

		if !resp.Ok {
			if resp.StatusCode == 429 {
				i += 1
				if i > api.opt.FloodRetries {
					return nil, &tlsclient.FloodWaitError{
						StatusCode: 429,
						RetryAfter: t.Seconds(),
						Origin:     "portals-market.com",
					}
				}

				log.Printf("INFO: portals-market.com returned 429, waiting for %f secs...", t.Seconds())
				time.Sleep(t)
				t *= 2
				continue
			}

			return nil, fmt.Errorf("%d: %s", resp.StatusCode, string(resp.Body))
		}

		break
	}

	var prices *FloorPrices
	if err := json.Unmarshal(resp.Body, &prices); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %v", err)
	}

	return prices, nil
}
