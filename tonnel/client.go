package tonnel

import (
	"autobid/tlsclient"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"time"
)

type TonnelAPI struct {
	conn *tlsclient.TLSClient
	opt  *Options
}

type Options struct {
	Proxies      []*url.URL
	FloodRetries uint32
}

func New(opt *Options) (*TonnelAPI, error) {
	if opt == nil {
		opt = &Options{}
	}

	c, err := tlsclient.New("rs-gifts.tonnel.network", false, false, opt.Proxies)
	if err != nil {
		return nil, err
	}
	return &TonnelAPI{conn: c, opt: opt}, nil
}

const FLOOD_WAIT time.Duration = time.Second * 5

var DEFAULT_HEADERS = map[string]string{
	"accept":     "application/json, text/plain, */*",
	"user-agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36",
	"connection": "keep-alive",
}

type RequestBody struct {
	Page       uint32      `json:"page"`
	Limit      uint32      `json:"limit"`
	Sort       string      `json:"sort"`
	Filter     string      `json:"filter"`
	Ref        int         `json:"ref"`
	PriceRange interface{} `json:"price_range"`
	UserAuth   string      `json:"user_auth"`
}

type BidHistoryEntry struct {
	Bidder    float64   `json:"bidder"`
	Amount    float64   `json:"amount"`
	Asset     string    `json:"asset"`
	ID        string    `json:"_id"`
	Timestamp time.Time `json:"timestamp"`
}

type Auction struct {
	ID               string            `json:"_id"`
	GiftID           int               `json:"gift_id"`
	Seller           int               `json:"seller"`
	AuctionID        string            `json:"auction_id"`
	StartingBid      float64           `json:"startingBid"`
	BidHistory       []BidHistoryEntry `json:"bidHistory"`
	AuctionEndTime   time.Time         `json:"auctionEndTime"`
	Status           string            `json:"status"`
	GiftName         string            `json:"gift_name"`
	GiftNum          int               `json:"gift_num"`
	Model            string            `json:"model"`
	Backdrop         string            `json:"backdrop"`
	Symbol           string            `json:"symbol"`
	Asset            string            `json:"asset"`
	AuctionStartTime time.Time         `json:"auctionStartTime"`
	V                int               `json:"__v"`
}

type Gift struct {
	GiftNum            int                    `json:"gift_num"`
	GiftID             int                    `json:"gift_id"`
	Name               string                 `json:"name"`
	Model              string                 `json:"model"`
	Asset              string                 `json:"asset"`
	Symbol             string                 `json:"symbol"`
	Backdrop           string                 `json:"backdrop"`
	AvailabilityIssued int                    `json:"availabilityIssued"`
	AvailabilityTotal  int                    `json:"availabilityTotal"`
	BackdropData       map[string]interface{} `json:"backdropData"`
	MessageInChannel   int                    `json:"message_in_channel"`
	Status             string                 `json:"status"`
	Limited            bool                   `json:"limited"`
	AuctionID          string                 `json:"auction_id"`
	Auction            *Auction               `json:"auction"` // pointer since auction may be null
	ExportAt           time.Time              `json:"export_at"`
	CustomEmojiID      string                 `json:"customEmojiId"`
	PremarketData      map[string]interface{} `json:"premarketData"`
	Price              float64                `json:"price,omitempty"` // optional field
}

func (g *Gift) MinBid() float64 {
	bid_step := 0.05
	if len(g.Auction.BidHistory) > 0 {
		highest_bid := g.Auction.BidHistory[len(g.Auction.BidHistory)-1]
		step := highest_bid.Amount * bid_step
		return highest_bid.Amount + step
	}

	return g.Auction.StartingBid
}

func (api *TonnelAPI) GetFloor(ctx context.Context, giftName string, model string, backdrop string) (*Gift, error) {
	url := "https://rs-gifts.tonnel.network/api/pageGifts"

	headers := make(map[string]string, len(DEFAULT_HEADERS))
	for k, v := range DEFAULT_HEADERS {
		headers[k] = v
	}
	headers["Content-Type"] = "application/json"

	filterMap := map[string]interface{}{
		"price":     map[string]interface{}{"$exists": true},
		"buyer":     map[string]interface{}{"$exists": false},
		"gift_name": giftName,
		"asset":     "TON",
	}
	if len(model) > 0 {
		filterMap["model"] = model
	}
	if len(backdrop) > 0 {
		filterMap["backdrop"] = map[string]interface{}{"$in": []string{backdrop}}
	}
	filterBytes, err := json.Marshal(filterMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal filter: %w", err)
	}
	bodyStruct := RequestBody{
		Page:       1,
		Limit:      30,
		Sort:       `{"price":1,"gift_id":-1}`,
		Filter:     string(filterBytes),
		Ref:        0,
		PriceRange: nil,
		UserAuth:   "",
	}
	bodyBytes, err := json.Marshal(bodyStruct)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	var resp *tlsclient.RequestResponse
	i := uint32(0)
	t := FLOOD_WAIT
	for {
		resp, err = api.conn.Request(ctx, "POST", url, bytes.NewReader(bodyBytes), headers, 3, 60*time.Second)
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
						Origin:     "rs-gifts.tonnel.network",
					}
				}

				log.Printf("INFO: rs-gifts.tonnel.network returned 429, waiting for %f secs...", t.Seconds())
				time.Sleep(t)
				t *= 2
				continue
			}

			return nil, fmt.Errorf("%d: %s", resp.StatusCode, string(resp.Body))
		}

		break
	}

	var gifts []Gift
	if err := json.Unmarshal(resp.Body, &gifts); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %v", err)
	}

	if len(gifts) < 1 {
		return nil, nil
	}

	return &gifts[0], nil
}

func (api *TonnelAPI) GetAuctions(ctx context.Context, page uint32, limit uint32) ([]Gift, error) {
	url := "https://rs-gifts.tonnel.network/api/pageGifts"

	headers := make(map[string]string, len(DEFAULT_HEADERS))
	for k, v := range DEFAULT_HEADERS {
		headers[k] = v
	}
	headers["Content-Type"] = "application/json"
	bodyStruct := RequestBody{
		Page:       page,
		Limit:      limit,
		Sort:       `{"auctionEndTime":1,"gift_id":-1}`,
		Filter:     `{"auction_id":{"$exists":true},"status":"active","asset":"TON"}`,
		Ref:        0,
		PriceRange: nil,
		UserAuth:   "",
	}
	bodyBytes, err := json.Marshal(bodyStruct)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	var resp *tlsclient.RequestResponse
	i := uint32(0)
	t := FLOOD_WAIT
	for {
		resp, err = api.conn.Request(ctx, "POST", url, bytes.NewReader(bodyBytes), headers, 3, 60*time.Second)
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
						Origin:     "rs-gifts.tonnel.network",
					}
				}

				log.Printf("INFO: rs-gifts.tonnel.network returned 429, waiting for %f secs...", t.Seconds())
				time.Sleep(t)
				t *= 2
				continue
			}

			return nil, fmt.Errorf("%d: %s", resp.StatusCode, string(resp.Body))
		}

		break
	}

	var gifts []Gift
	if err := json.Unmarshal(resp.Body, &gifts); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %v", err)
	}

	return gifts, nil
}
