package ip

import (
	"autobid/tlsclient"
	"fmt"
	"log"
	"net/url"
	"time"
)

type IpifyAPI struct {
	conn *tlsclient.TLSClient
	opt  *Options
}

type Options struct {
	Proxies      []*url.URL
	FloodRetries uint32
}

func New(opt *Options) (*IpifyAPI, error) {
	if opt == nil {
		opt = &Options{}
	}

	c, err := tlsclient.New("api.ipify.org", false, false, opt.Proxies)
	if err != nil {
		return nil, err
	}
	return &IpifyAPI{conn: c, opt: opt}, nil
}

const FLOOD_WAIT time.Duration = time.Second * 5

var DEFAULT_HEADERS = map[string]string{
	"accept":     "application/json, text/plain, */*",
	"user-agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36",
}

func (api *IpifyAPI) GetIp() (string, error) {
	url := "https://api.ipify.org"

	headers := make(map[string]string, len(DEFAULT_HEADERS))
	for k, v := range DEFAULT_HEADERS {
		headers[k] = v
	}

	var err error
	var resp *tlsclient.RequestResponse
	i := uint32(0)
	t := FLOOD_WAIT
	for {
		resp, err = api.conn.Request("GET", url, nil, headers, 3)
		if err != nil {
			return "", err
		}

		if !resp.Ok {
			if resp.StatusCode == 429 {
				i += 1
				if i > api.opt.FloodRetries {
					return "", &tlsclient.FloodWaitError{
						StatusCode: 429,
						RetryAfter: t.Seconds(),
						Origin:     "api.ipify.org",
					}
				}

				log.Printf("INFO: api.ipify.org returned 429, waiting for %f secs...", t.Seconds())
				time.Sleep(t)
				t *= 2
				continue
			}

			return "", fmt.Errorf("%d: %s", resp.StatusCode, string(resp.Body))
		}

		break
	}

	return string(resp.Body), nil
}
