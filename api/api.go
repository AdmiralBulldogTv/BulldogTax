package api

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/pasztorpisti/qs"
	"github.com/troydota/bulldog-taxes/auth"
	"github.com/troydota/bulldog-taxes/configure"
	"github.com/troydota/bulldog-taxes/utils"

	jsoniter "github.com/json-iterator/go"
	log "github.com/sirupsen/logrus"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type TwitchUserResp struct {
	Data []TwitchUser `json:"data"`
}

type TwitchUser struct {
	ID              string    `json:"id"`
	Login           string    `json:"login"`
	DisplayName     string    `json:"display_name"`
	BroadcasterType string    `json:"broadcaster_type"`
	Description     string    `json:"description"`
	ProfileImageURL string    `json:"profile_image_url"`
	OfflineImageURL string    `json:"offline_image_url"`
	ViewCount       int       `json:"view_count"`
	CreatedAt       time.Time `json:"created_at"`
}

func GetUsers(ctx context.Context, oauth string, ids []string, logins []string) ([]TwitchUser, error) {
	returnv := []TwitchUser{}
	for len(ids) != 0 || len(logins) != 0 {
		var temp []string
		var temp2 []string
		if len(ids) > 100 {
			temp = ids[:100]
			ids = ids[100:]
		} else {
			temp = ids
			ids = []string{}
			if len(logins)+len(temp) > 100 {
				temp2 = logins[:100-len(temp)]
				logins = logins[100-len(temp):]
			} else {
				temp2 = logins
				logins = []string{}
			}
		}

		params, _ := qs.Marshal(map[string][]string{
			"id":    temp,
			"login": temp2,
		})

		u, _ := url.Parse(fmt.Sprintf("https://api.twitch.tv/helix/users?%s", params))

		var token string
		var err error

		if oauth == "" {
			token, err = auth.GetAuth(ctx)
			if err != nil {
				return nil, err
			}
		} else {
			token = oauth
		}

		resp, err := http.DefaultClient.Do(&http.Request{
			Method: "GET",
			URL:    u,
			Header: http.Header{
				"Client-Id":     []string{configure.Config.GetString("twitch_client_id")},
				"Authorization": []string{fmt.Sprintf("Bearer %s", token)},
			},
		})
		if err != nil {
			return nil, err
		}

		defer resp.Body.Close()

		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		respData := TwitchUserResp{}

		if err := json.Unmarshal(data, &respData); err != nil {
			return nil, err
		}
		returnv = append(returnv, respData.Data...)
	}

	if oauth != "" && len(ids) == 0 && len(logins) == 0 {
		u, _ := url.Parse("https://api.twitch.tv/helix/users")

		var err error

		resp, err := http.DefaultClient.Do(&http.Request{
			Method: "GET",
			URL:    u,
			Header: http.Header{
				"Client-Id":     []string{configure.Config.GetString("twitch_client_id")},
				"Authorization": []string{fmt.Sprintf("Bearer %s", oauth)},
			},
		})
		if err != nil {
			return nil, err
		}

		defer resp.Body.Close()

		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		respData := TwitchUserResp{}

		if err := json.Unmarshal(data, &respData); err != nil {
			return nil, err
		}
		return respData.Data, nil
	}

	return returnv, nil
}

// {
//     "type": "channel.ban",
//     "version": "1",
//     "condition": {
//         "broadcaster_user_id": "121903137"
//     },
//     "transport": {
//         "method": "webhook",
//         "callback": "https://modlogs.komodohype.dev/webhook/channel.ban/121903137",
//         "secret": "5353208469b4788087d51f2a167fdf7b338f40af20cc05f8f65dacbdf792ee92"
//     }
// }

type TwitchWebhookRequest struct {
	Type      string                  `json:"type"`
	Version   string                  `json:"version"`
	Condition map[string]interface{}  `json:"condition"`
	Transport TwitchCallbackTransport `json:"transport"`
}

type TwitchWebhookRequestResp struct {
	Data []TwitchWebhookRequestRespData `json:"data"`
}

type TwitchWebhookRequestRespData struct {
	ID string `json:"id"`
}

type TwitchCallbackTransport struct {
	Method   string `json:"method"`
	Callback string `json:"callback"`
	Secret   string `json:"secret"`
}

func CreateWebhook(ctx context.Context, streamerID string) (string, string, error) {
	secret, err := utils.GenerateRandomString(64)
	if err != nil {
		return "", "", err
	}

	token, err := auth.GetAuth(ctx)
	if err != nil {
		return "", "", err
	}

	data, err := json.Marshal(TwitchWebhookRequest{
		Type:    "channel.channel_points_custom_reward_redemption.add",
		Version: "1",
		Condition: map[string]interface{}{
			"broadcaster_user_id": streamerID,
		},
		Transport: TwitchCallbackTransport{
			Method:   "webhook",
			Callback: fmt.Sprintf("%s/webhook/%s", configure.Config.GetString("website_url"), streamerID),
			Secret:   secret,
		},
	})
	if err != nil {
		return "", "", err
	}
	req, err := http.NewRequest("POST", "https://api.twitch.tv/helix/eventsub/subscriptions", bytes.NewBuffer(data))
	if err != nil {
		log.Errorf("req, err=%e", err)
		return "", "", err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Client-ID", configure.Config.GetString("twitch_client_id"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Errorf("resp, err=%e", err)
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode > 300 {
		data, err := ioutil.ReadAll(resp.Body)
		log.Errorf("twitch, body=%s, err=%e", data, err)
		return "", "", fmt.Errorf("invalid resp from twitch")
	}

	respDataRaw, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	respData := &TwitchWebhookRequestResp{}
	if err = json.Unmarshal(respDataRaw, respData); err != nil {
		return "", "", err
	}

	return respData.Data[0].ID, secret, nil
}

func RevokeWebhook(ctx context.Context, webhookID string) error {
	token, err := auth.GetAuth(ctx)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("DELETE", fmt.Sprintf("https://api.twitch.tv/helix/eventsub/subscriptions?id=%s", webhookID), nil)
	if err != nil {
		log.Errorf("req, err=%e", err)
		return err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Client-ID", configure.Config.GetString("twitch_client_id"))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Errorf("resp, err=%e", err)
		return err
	}

	if resp.StatusCode > 300 {
		data, err := ioutil.ReadAll(resp.Body)
		log.Errorf("twitch, body=%s, err=%e", data, err)
		return fmt.Errorf("invalid resp from twitch")
	}

	return nil
}
