package surveyor

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Replace these constants with your actual endpoint and credentials
const (
	username = "admin"
	password = "Google.com1"
	hnapURL  = "https://192.168.100.1/HNAP1/"

	loginAction          = `"http://purenetworks.com/HNAP1/Login"`
	getChannelInfoAction = `"http://purenetworks.com/HNAP1/GetMultipleHNAPs"`
)

var notFound = errors.New("not found")

type LoginRequest struct {
	Login LoginRequestBody `json:"Login"`
}

type LoginRequestBody struct {
	Action        string `json:"Action"`
	Username      string `json:"Username"`
	LoginPassword string `json:"LoginPassword"`
	Captcha       string `json:"Captcha"`
	PrivateLogin  string `json:"PrivateLogin"`
}

func NewLoginRequest(action, username, password string) LoginRequest {
	return LoginRequest{
		Login: LoginRequestBody{
			Action:        action,
			Username:      username,
			LoginPassword: password,
			PrivateLogin:  "LoginPassword",
		},
	}
}

type LoginResponse struct {
	LoginResponse LoginResponseBody `json:"LoginResponse"`
}

type LoginResponseBody struct {
	Challenge   string `json:"Challenge"`
	Cookie      string `json:"Cookie"`
	PublicKey   string `json:"PublicKey"`
	LoginResult string `json:"LoginResult"`
}

type Challenge struct {
	PublicKey, UID, Message string
}

func NewChallenge(loginResponse LoginResponse) Challenge {
	body := loginResponse.LoginResponse
	return Challenge{
		PublicKey: body.PublicKey,
		UID:       body.Cookie,
		Message:   body.Challenge,
	}
}

type Credentials struct {
	UID, PrivateKey, Username, Password string
}

func NewCredentials(challenge Challenge, username, password string) Credentials {
	challengeKey := challenge.PublicKey + password
	privateKey := CalculateHMAC(challenge.Message, challengeKey)
	pass := CalculateHMAC(challenge.Message, privateKey)

	return Credentials{
		UID:        challenge.UID,
		PrivateKey: privateKey,
		Username:   username,
		Password:   pass,
	}
}

func (creds Credentials) Empty() bool {
	return creds.UID == "" || creds.PrivateKey == ""
}

type GetMultipleHNAPs struct {
	Body struct {
		GetCustomerStatusDownstreamChannelInfo string `json:"GetCustomerStatusDownstreamChannelInfo"`
		//GetCustomerStatusUpstreamChannelInfo   string `json:"GetCustomerStatusUpstreamChannelInfo"`
	} `json:"GetMultipleHNAPs"`
}

type GetCustomerStatusDownstreamChannelInfoResponse struct {
	Downstream struct {
		Result   string `json:"GetCustomerStatusDownstreamChannelInfoResult"`
		Channels string `json:"CustomerConnDownstreamChannel"`
	} `json:"GetCustomerStatusDownstreamChannelInfoResponse"`
}

type GetMultipleHNAPsResponse struct {
	Body struct {
		Result     string `json:"GetMultipleHNAPsResult"`
		Downstream struct {
			Result   string `json:"GetCustomerStatusDownstreamChannelInfoResult"`
			Channels string `json:"CustomerConnDownstreamChannel"`
		} `json:"GetCustomerStatusDownstreamChannelInfoResponse"`
		Upstream struct {
			Result   string `json:"GetCustomerStatusUpstreamChannelInfoResult"`
			Channels string `json:"CustomerConnUpstreamChannel"`
		} `json:"GetCustomerStatusUpstreamChannelInfoResponse"`
	} `json:"GetMultipleHNAPsResponse"`
}

type HNAPClient struct {
	client      http.Client
	credentials Credentials
}

func NewHNAPClient() *HNAPClient {
	client := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	return &HNAPClient{client: client}
}

func (client *HNAPClient) GetSignalData(ctx context.Context) (SignalData, error) {
	var resp GetCustomerStatusDownstreamChannelInfoResponse
	var err error

	resp, err = client.attemptGetSignalData(ctx)
	if errors.Is(err, notFound) {
		client.credentials = Credentials{}
		resp, err = client.attemptGetSignalData(ctx)
	}

	if err != nil {
		return nil, err
	}

	if resp.Downstream.Result != "OK" {
		return nil, fmt.Errorf("error in downstream info, result=%q", resp.Downstream.Result)
	}

	return ChannelInfosToSignalData(resp.Downstream.Channels)
}

func (client *HNAPClient) attemptGetSignalData(ctx context.Context) (GetCustomerStatusDownstreamChannelInfoResponse, error) {
	if client.credentials.Empty() {
		fmt.Println("credentials empty, logging in")
		if err := client.Login(ctx); err != nil {
			return GetCustomerStatusDownstreamChannelInfoResponse{}, err
		}
	}

	return client.RequestStreamInfos(ctx)
}

func (client *HNAPClient) Login(ctx context.Context) error {
	challenge, err := client.GetChallenge(ctx, username)
	if err != nil {
		return err
	}

	client.credentials = NewCredentials(challenge, username, password)

	if err := client.SubmitChallenge(ctx); err != nil {
		return err
	}

	return nil
}

func (client *HNAPClient) GetChallenge(ctx context.Context, username string) (Challenge, error) {
	request := NewLoginRequest("request", username, "")
	body, err := client.MakeRequest(ctx, request, loginAction)
	if err != nil {
		return Challenge{}, err
	}

	var loginResponse LoginResponse
	if err := json.Unmarshal(body, &loginResponse); err != nil {
		return Challenge{}, fmt.Errorf("error unmarshalling json: %w", err)
	}

	return NewChallenge(loginResponse), nil
}

func (client *HNAPClient) SubmitChallenge(ctx context.Context) error {
	request := NewLoginRequest("login", client.credentials.Username, client.credentials.Password)
	_, err := client.MakeRequest(ctx, request, loginAction)
	if err != nil {
		return err
	}

	return nil
}

func (client *HNAPClient) RequestStreamInfos(ctx context.Context) (GetCustomerStatusDownstreamChannelInfoResponse, error) {
	body, err := client.MakeRequest(ctx, GetMultipleHNAPs{}, getChannelInfoAction)
	if err != nil {
		return GetCustomerStatusDownstreamChannelInfoResponse{}, err
	}

	var response GetCustomerStatusDownstreamChannelInfoResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return GetCustomerStatusDownstreamChannelInfoResponse{}, fmt.Errorf("error unmarshalling json: %w", err)
	}

	return response, nil
}

func (client *HNAPClient) MakeRequest(ctx context.Context, request any, action string) ([]byte, error) {
	payloadBytes, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("error marshaling json: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", hnapURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	for key, value := range HNAPHeaders(action, client.credentials.PrivateKey, client.credentials.UID, time.Now()) {
		req.Header.Set(key, value)
	}

	resp, err := client.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error performing request: %w", err)
	}
	defer ClosePrintErr(resp.Body)

	if resp.StatusCode == 404 {
		return nil, notFound
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("received http error status=%d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	return body, nil
}

func CalculateHMAC(message, key string) string {
	h := hmac.New(md5.New, []byte(key))
	h.Write([]byte(message))
	encoded := strings.ToUpper(hex.EncodeToString(h.Sum(nil)))
	return encoded
}

func HNAPHeaders(action, privateKey, uid string, now time.Time) map[string]string {
	headers := make(map[string]string)

	if privateKey == "" {
		privateKey = "withoutloginkey"
	}

	// Can't wait to see if this code works after May 2033
	currentTimeMS := now.Unix() % 2_000_000_000_000
	message := strconv.FormatInt(currentTimeMS, 10) + action

	encoded := CalculateHMAC(message, privateKey)
	hnapAuth := fmt.Sprintf("%s %d", encoded, currentTimeMS)

	headers["SOAPACTION"] = action
	headers["HNAP_AUTH"] = hnapAuth

	if uid != "" {
		cookie := fmt.Sprintf("Secure; Secure; uid=%s; PrivateKey=%s", uid, privateKey)
		headers["Cookie"] = cookie
	}

	return headers
}
