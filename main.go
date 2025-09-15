package main

import (
	"autobid/config"
	"autobid/telegram"
	"autobid/tonnel"
	"context"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

func main() {
	cfg, err := config.LoadConfig(".")
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}
	proxies := []*url.URL{}
	for _, proxyStr := range cfg.Proxies {
		proxy, err := url.Parse(proxyStr)
		if err != nil {
			log.Fatalf("invalid proxy address: %s\n", err)
		}
		proxies = append(proxies, proxy)
	}

	tgLogger := telegram.NewLogger(cfg.Token, cfg.ChatID)
	client, err := tonnel.New(&tonnel.Options{
		Proxies: proxies,
	})
	if err != nil {
		log.Fatalf("connection to tonnel failed: %v", err)
	}

	for {
		log.Printf("fetching auctions...")
		gifts, err := client.GetAuctions(1, cfg.GiftsPerFetch)
		if err != nil {
			log.Fatalf("error GetAuctions: %v", err)
		}

		var latest time.Time
		filteredGifts := []tonnel.Gift{}
		for _, g := range gifts {
			now := time.Now()
			end := g.Auction.AuctionEndTime
			if now.After(end) || g.GiftID < 0 {
				continue
			}
			if end.After(latest) {
				latest = end
			}
			filteredGifts = append(filteredGifts, g)
		}
		earliest := latest
		for _, g := range filteredGifts {
			end := g.Auction.AuctionEndTime
			if end.Before(earliest) {
				earliest = end
			}
		}

		log.Printf("found %d auctions (%fs - %fs)", len(filteredGifts), time.Until(earliest).Seconds(), time.Until(latest).Seconds())
		ch := giftFloorGenerator(filteredGifts, proxies, cfg.RareBackdrops, cfg.ConcurrentRequests)
		for gf := range ch {
			if gf.Err != nil {
				log.Printf("error GetFloor gift %d: %v", gf.Gift.GiftID, gf.Err)
				continue
			}

			now := time.Now()
			end := gf.Gift.Auction.AuctionEndTime
			if now.After(end) {
				continue
			}

			bid := gf.Gift.MinBid()
			profit := gf.Floor - bid
			profitPercentage := 1 - (bid / gf.Floor)
			log.Printf("[%d] %s #%d = %f %s | %f %s (%f%% - %fs)\n", gf.Gift.GiftID, gf.Gift.Name, gf.Gift.GiftNum, bid, gf.Gift.Asset, gf.Floor, gf.Gift.Asset, profitPercentage*100, time.Until(end).Seconds())

			if profitPercentage < cfg.MinProfit || profit < cfg.MinProfitTon {
				continue
			}

			d := time.Until(end)
			hours := int(d / time.Hour)
			d -= time.Duration(hours) * time.Hour
			minutes := int(d / time.Minute)
			d -= time.Duration(minutes) * time.Minute
			seconds := int(d / time.Second)
			re := regexp.MustCompile(`\s+`)
			link := fmt.Sprintf("https://t.me/nft/%s-%d", re.ReplaceAllString(gf.Gift.Name, ""), gf.Gift.GiftNum)
			msg := fmt.Sprintf("<a href=\"%s\">%s #%d</a>\n\nBid Cost: <b>%f</b> %s\nMin Sell: <b>%f</b> %s\nProfit: <b>%f</b>%% (%f %s)\nEnd in: %02d:%02d:%02d\n\n<a href=\"https://t.me/tonnel_network_bot/gift?startapp=%d\">Link</a>", link, gf.Gift.Name, gf.Gift.GiftNum, bid, gf.Gift.Asset, gf.Floor, gf.Gift.Asset, profitPercentage*100, gf.Floor-bid, gf.Gift.Asset, hours, minutes, seconds, gf.Gift.GiftID)
			go tgLogger.SendMessage(context.Background(), msg, true, nil)
		}

		t := time.Until(latest)
		log.Printf("waiting for %f secs for new auctions...\n", t.Seconds())
		time.Sleep(t)
	}
}

type GiftWithFloor struct {
	Gift  tonnel.Gift
	Floor float64
	Err   error
}

func giftFloorGenerator(gifts []tonnel.Gift, proxies []*url.URL, rare_backdrops []string, maxConcurrent int) <-chan GiftWithFloor {
	out := make(chan GiftWithFloor)
	go func() {
		defer close(out)

		sem := make(chan struct{}, maxConcurrent)
		var wg sync.WaitGroup

		for _, g := range gifts {
			wg.Add(1)
			go func(g tonnel.Gift) {
				defer wg.Done()

				sem <- struct{}{}
				floor, err := getFloor(proxies, g.Name, g.Model, g.Backdrop, rare_backdrops)
				<-sem

				out <- GiftWithFloor{Gift: g, Floor: floor, Err: err}
			}(g)
		}

		wg.Wait()
	}()
	return out
}

func getFloor(proxies []*url.URL, giftName, model, backdrop string, rare_backdrops []string) (float64, error) {
	re := regexp.MustCompile(`\s*\([^)]*%?\)`)
	output := re.ReplaceAllString(backdrop, "")
	filterModel := model
	filterBackdrop := ""
	lowerOutput := strings.ToLower(output)

	for _, rb := range rare_backdrops {
		if strings.ToLower(rb) == lowerOutput {
			filterModel = ""
			filterBackdrop = backdrop
			break
		}
	}

	client, err := tonnel.New(&tonnel.Options{
		Proxies: proxies,
	})
	if err != nil {
		return 0, err
	}

	gift, err := client.GetFloor(giftName, filterModel, filterBackdrop)
	if err != nil {
		return 0, err
	}
	if gift == nil {
		gift, err = client.GetFloor(giftName, "", "")
		if err != nil {
			return 0, err
		}
	}

	return gift.Price, nil
}
