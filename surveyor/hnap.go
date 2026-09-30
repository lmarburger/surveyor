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
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultURL      = "https://192.168.100.1/HNAP1/"
	DefaultUsername = "admin"

	loginAction    = `"http://purenetworks.com/HNAP1/Login"`
	multipleAction = `"http://purenetworks.com/HNAP1/GetMultipleHNAPs"`

	resultOK = "OK"
)

const (
	StageLogin = "login"
	StageFetch = "fetch"
	StageParse = "parse"
)

// The modem answers 404 to any request carrying a missing or expired session.
var errNotFound = errors.New("not found")

var ErrLoginFailed = errors.New("modem rejected login")

// FetchError records which step of talking to the modem failed, so callers can
// tell a bad password apart from a slow or unreachable modem.
type FetchError struct {
	Stage string
	Err   error
}

func (e *FetchError) Error() string { return e.Stage + ": " + e.Err.Error() }
func (e *FetchError) Unwrap() error { return e.Err }

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

type statusRequest struct {
	Body struct {
		Downstream string `json:"GetCustomerStatusDownstreamChannelInfo"`
		Upstream   string `json:"GetCustomerStatusUpstreamChannelInfo"`
		Software   string `json:"GetCustomerStatusSoftware"`
	} `json:"GetMultipleHNAPs"`
}

type statusResponse struct {
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
		Software struct {
			Result          string `json:"GetCustomerStatusSoftwareResult"`
			SpecVersion     string `json:"StatusSoftwareSpecVer"`
			HardwareVersion string `json:"StatusSoftwareHdVer"`
			SoftwareVersion string `json:"StatusSoftwareSfVer"`
		} `json:"GetCustomerStatusSoftwareResponse"`
	} `json:"GetMultipleHNAPsResponse"`
}

type ModemInfo struct {
	DOCSIS, Hardware, Firmware string
}

type Status struct {
	Info       ModemInfo
	Downstream []DownstreamChannel
	Upstream   []UpstreamChannel
	// Records the modem sent that could not be parsed and were left out.
	SkippedDownstream, SkippedUpstream int
}

// HNAPClient is not safe for concurrent use. The modem copes badly with
// overlapping requests anyway (it drops connections), so callers should
// serialize access rather than share a client.
type HNAPClient struct {
	client      http.Client
	url         string
	username    string
	password    string
	credentials Credentials
	now         func() time.Time
}

func NewHNAPClient(url, username, password string) *HNAPClient {
	return &HNAPClient{
		client: http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				MaxConnsPerHost: 1,
			},
		},
		url:      url,
		username: username,
		password: password,
		now:      time.Now,
	}
}

// Fetch returns the modem's current channel status. On any failure the session
// is discarded, so the next call starts from a fresh login instead of retrying
// a session the modem may no longer honor.
func (client *HNAPClient) Fetch(ctx context.Context) (Status, error) {
	status, err := client.attemptFetch(ctx)
	if errors.Is(err, errNotFound) {
		slog.Info("modem session expired, logging in again")
		client.credentials = Credentials{}
		status, err = client.attemptFetch(ctx)
	}
	if err != nil {
		client.credentials = Credentials{}
		return Status{}, err
	}
	return status, nil
}

func (client *HNAPClient) attemptFetch(ctx context.Context) (Status, error) {
	if client.credentials.Empty() {
		if err := client.Login(ctx); err != nil {
			return Status{}, &FetchError{Stage: StageLogin, Err: err}
		}
	}

	body, err := client.MakeRequest(ctx, statusRequest{}, multipleAction)
	if errors.Is(err, errNotFound) {
		return Status{}, err
	}
	if err != nil {
		return Status{}, &FetchError{Stage: StageFetch, Err: err}
	}

	status, err := parseStatus(body)
	if err != nil {
		return Status{}, &FetchError{Stage: StageParse, Err: err}
	}
	return status, nil
}

func parseStatus(body []byte) (Status, error) {
	var resp statusResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return Status{}, fmt.Errorf("error unmarshalling json: %w", err)
	}

	r := resp.Body
	if r.Result != resultOK {
		return Status{}, fmt.Errorf("modem returned result=%q", r.Result)
	}
	if r.Downstream.Result != resultOK {
		return Status{}, fmt.Errorf("modem returned downstream result=%q", r.Downstream.Result)
	}

	var status Status
	status.Downstream, status.SkippedDownstream = ParseDownstream(r.Downstream.Channels)

	// Upstream and software info are extras. A failure there should not throw
	// away the downstream data, which is the main point of the exporter.
	if r.Upstream.Result == resultOK {
		status.Upstream, status.SkippedUpstream = ParseUpstream(r.Upstream.Channels)
	}
	if r.Software.Result == resultOK {
		status.Info = ModemInfo{
			DOCSIS:   r.Software.SpecVersion,
			Hardware: r.Software.HardwareVersion,
			Firmware: r.Software.SoftwareVersion,
		}
	}
	return status, nil
}

func (client *HNAPClient) Login(ctx context.Context) error {
	client.credentials = Credentials{}

	challenge, err := client.GetChallenge(ctx, client.username)
	if err != nil {
		return err
	}

	creds := NewCredentials(challenge, client.username, client.password)
	if err := client.SubmitChallenge(ctx, creds); err != nil {
		return err
	}

	client.credentials = creds
	slog.Info("logged in to modem")
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
	if result := loginResponse.LoginResponse.LoginResult; result != resultOK {
		return Challenge{}, fmt.Errorf("%w: challenge result=%q", ErrLoginFailed, result)
	}

	return NewChallenge(loginResponse), nil
}

// SubmitChallenge sends the hashed password. The modem answers a wrong
// password with HTTP 200 and LoginResult "FAILED", so the body must be checked.
func (client *HNAPClient) SubmitChallenge(ctx context.Context, creds Credentials) error {
	request := NewLoginRequest("login", creds.Username, creds.Password)
	body, err := client.makeRequest(ctx, request, loginAction, creds)
	if err != nil {
		return err
	}

	var loginResponse LoginResponse
	if err := json.Unmarshal(body, &loginResponse); err != nil {
		return fmt.Errorf("error unmarshalling json: %w", err)
	}
	if result := loginResponse.LoginResponse.LoginResult; result != resultOK {
		return fmt.Errorf("%w: result=%q", ErrLoginFailed, result)
	}
	return nil
}

func (client *HNAPClient) MakeRequest(ctx context.Context, request any, action string) ([]byte, error) {
	return client.makeRequest(ctx, request, action, client.credentials)
}

func (client *HNAPClient) makeRequest(ctx context.Context, request any, action string, creds Credentials) ([]byte, error) {
	payloadBytes, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("error marshaling json: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", client.url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	for key, value := range HNAPHeaders(action, creds.PrivateKey, creds.UID, client.now()) {
		req.Header.Set(key, value)
	}

	resp, err := client.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error performing request: %w", err)
	}
	defer ClosePrintErr(resp.Body)

	if resp.StatusCode == http.StatusNotFound {
		return nil, errNotFound
	}
	if resp.StatusCode != http.StatusOK {
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

	// Mirrors the modem's own web UI: `Date.now() % 2000000000000`.
	currentTimeMS := now.UnixMilli() % 2_000_000_000_000
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

func ClosePrintErr(body io.Closer) {
	err := body.Close()
	if err != nil {
		slog.Warn("error closing body", "err", err)
	}
}
