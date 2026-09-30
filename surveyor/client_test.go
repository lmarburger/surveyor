package surveyor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeModem mimics what the real modem was observed to do: a wrong password
// gets HTTP 200 with LoginResult "FAILED", and any request with an unknown
// session gets a 404.
type fakeModem struct {
	mu           sync.Mutex
	password     string
	challenges   map[string]Challenge
	sessions     map[string]bool
	nextID       int
	logins       int
	statusCalls  int
	statusResult string
	delay        time.Duration
}

func newFakeModem(t *testing.T, password string) (*fakeModem, *httptest.Server) {
	m := &fakeModem{
		password:     password,
		challenges:   map[string]Challenge{},
		sessions:     map[string]bool{},
		statusResult: "OK",
	}
	server := httptest.NewTLSServer(m)
	t.Cleanup(server.Close)
	return m, server
}

var cookieUID = regexp.MustCompile(`uid=([^;]+)`)

func (m *fakeModem) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	delay := m.delay
	m.mu.Unlock()
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-r.Context().Done():
			return
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	uid := ""
	if match := cookieUID.FindStringSubmatch(r.Header.Get("Cookie")); match != nil {
		uid = match[1]
	}

	switch r.Header.Get("SOAPACTION") {
	case loginAction:
		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		m.handleLogin(w, req.Login, uid)
	case multipleAction:
		if !m.sessions[uid] {
			http.NotFound(w, r)
			return
		}
		m.statusCalls++
		fmt.Fprintf(w, `{"GetMultipleHNAPsResponse": {
			"GetCustomerStatusDownstreamChannelInfoResponse": {"CustomerConnDownstreamChannel": %q, "GetCustomerStatusDownstreamChannelInfoResult": "OK"},
			"GetCustomerStatusUpstreamChannelInfoResponse": {"CustomerConnUpstreamChannel": %q, "GetCustomerStatusUpstreamChannelInfoResult": "OK"},
			"GetCustomerStatusSoftwareResponse": {"StatusSoftwareSpecVer": "DOCSIS 3.1", "StatusSoftwareHdVer": "V1.0", "StatusSoftwareSfVer": "TB01.01.001.14", "GetCustomerStatusSoftwareResult": "OK"},
			"GetMultipleHNAPsResult": %q}}`,
			`1^Locked^QAM256^32^741000000^2^34^39579^0^|+|2^Locked^Unknown^1^555000000^-47^0^0^0^`,
			`1^Locked^SC-QAM^13^6400000^37800000^38.8^`,
			m.statusResult)
	default:
		http.NotFound(w, r)
	}
}

func (m *fakeModem) handleLogin(w http.ResponseWriter, body LoginRequestBody, uid string) {
	switch body.Action {
	case "request":
		m.nextID++
		challenge := Challenge{
			PublicKey: fmt.Sprintf("public-%d", m.nextID),
			UID:       fmt.Sprintf("uid-%d", m.nextID),
			Message:   fmt.Sprintf("challenge-%d", m.nextID),
		}
		m.challenges[challenge.UID] = challenge
		fmt.Fprintf(w, `{"LoginResponse": {"Challenge": %q, "Cookie": %q, "PublicKey": %q, "LoginResult": "OK"}}`,
			challenge.Message, challenge.UID, challenge.PublicKey)
	case "login":
		challenge, ok := m.challenges[uid]
		expected := NewCredentials(challenge, body.Username, m.password)
		if !ok || body.LoginPassword != expected.Password {
			fmt.Fprint(w, `{"LoginResponse": {"LoginResult": "FAILED"}}`)
			return
		}
		m.logins++
		m.sessions[uid] = true
		fmt.Fprint(w, `{"LoginResponse": {"LoginResult": "OK"}}`)
	}
}

func (m *fakeModem) expireSessions() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions = map[string]bool{}
}

func (m *fakeModem) set(f func(m *fakeModem)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f(m)
}

func (m *fakeModem) counts() (logins, statusCalls int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.logins, m.statusCalls
}

func TestHNAPClient_Fetch(t *testing.T) {
	ctx := context.Background()

	t.Run("fetches status and reuses the session", func(t *testing.T) {
		modem, server := newFakeModem(t, "hunter2")
		client := NewHNAPClient(server.URL, "admin", "hunter2")

		status, err := client.Fetch(ctx)
		require.NoError(t, err)
		_, err = client.Fetch(ctx)
		require.NoError(t, err)

		assert.Len(t, status.Downstream, 2)
		assert.Equal(t, []UpstreamChannel{
			{ChannelID: 13, Locked: true, Type: "SC-QAM", SymbolRate: 6400000, FrequencyHz: 37800000, PowerDBmV: 38.8},
		}, status.Upstream)
		assert.Equal(t, ModemInfo{DOCSIS: "DOCSIS 3.1", Hardware: "V1.0", Firmware: "TB01.01.001.14"}, status.Info)

		logins, statusCalls := modem.counts()
		assert.Equal(t, 1, logins)
		assert.Equal(t, 2, statusCalls)
	})

	t.Run("reports a rejected password as a login failure", func(t *testing.T) {
		modem, server := newFakeModem(t, "hunter2")
		client := NewHNAPClient(server.URL, "admin", "wrong")

		_, err := client.Fetch(ctx)

		var fetchErr *FetchError
		require.ErrorAs(t, err, &fetchErr)
		assert.Equal(t, StageLogin, fetchErr.Stage)
		assert.ErrorIs(t, err, ErrLoginFailed)
		assert.True(t, client.credentials.Empty(), "a rejected login must not leave a session behind")
		_, statusCalls := modem.counts()
		assert.Zero(t, statusCalls)
	})

	t.Run("logs in again when the session expires", func(t *testing.T) {
		modem, server := newFakeModem(t, "hunter2")
		client := NewHNAPClient(server.URL, "admin", "hunter2")

		_, err := client.Fetch(ctx)
		require.NoError(t, err)
		modem.expireSessions()
		_, err = client.Fetch(ctx)
		require.NoError(t, err)

		logins, _ := modem.counts()
		assert.Equal(t, 2, logins)
	})

	t.Run("discards the session after a bad result", func(t *testing.T) {
		modem, server := newFakeModem(t, "hunter2")
		client := NewHNAPClient(server.URL, "admin", "hunter2")
		modem.set(func(m *fakeModem) { m.statusResult = "ERROR" })

		_, err := client.Fetch(ctx)
		var fetchErr *FetchError
		require.ErrorAs(t, err, &fetchErr)
		assert.Equal(t, StageParse, fetchErr.Stage)
		assert.True(t, client.credentials.Empty())

		modem.set(func(m *fakeModem) { m.statusResult = "OK" })
		_, err = client.Fetch(ctx)
		require.NoError(t, err)
		logins, _ := modem.counts()
		assert.Equal(t, 2, logins)
	})

	t.Run("discards the session after a timeout", func(t *testing.T) {
		modem, server := newFakeModem(t, "hunter2")
		client := NewHNAPClient(server.URL, "admin", "hunter2")
		_, err := client.Fetch(ctx)
		require.NoError(t, err)

		modem.set(func(m *fakeModem) { m.delay = time.Second })
		timeoutCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()
		_, err = client.Fetch(timeoutCtx)

		var fetchErr *FetchError
		require.ErrorAs(t, err, &fetchErr)
		assert.Equal(t, StageFetch, fetchErr.Stage)
		assert.True(t, errors.Is(err, context.DeadlineExceeded))
		assert.True(t, client.credentials.Empty())
	})
}
