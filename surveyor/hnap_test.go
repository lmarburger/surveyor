package surveyor

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestCalculateHMAC(t *testing.T) {
	t.Run("returns message encoded with key", func(t *testing.T) {
		encoded := CalculateHMAC("omg", "wtf")
		expected := "772EF7B15E93B46FFA669B78266BA283"
		assert.Equal(t, expected, encoded)
	})
}

func TestHNAPHeaders(t *testing.T) {
	t.Run("returns HNAP headers for given key and uid", func(t *testing.T) {
		action := `"my special action"`
		key := "supersecret"
		uid := "abc123"
		now := time.Unix(1710688116, 0)
		expected := map[string]string{
			"SOAPACTION": action,
			"HNAP_AUTH":  "8DD28DA21F6114D5E4DE5C0C4909A1C5 1710688116",
			"Cookie":     "Secure; Secure; uid=abc123; PrivateKey=supersecret",
		}

		headers := HNAPHeaders(action, key, uid, now)
		assert.Equal(t, expected, headers)
	})

	t.Run("returns HNAP headers for given key", func(t *testing.T) {
		action := `"my special action"`
		key := "supersecret"
		now := time.Unix(1710688116, 0)
		expected := map[string]string{
			"SOAPACTION": action,
			"HNAP_AUTH":  "8DD28DA21F6114D5E4DE5C0C4909A1C5 1710688116",
		}

		headers := HNAPHeaders(action, key, "", now)
		assert.Equal(t, expected, headers)
	})

	t.Run("returns HNAP headers using known private key", func(t *testing.T) {
		action := `"my special action"`
		now := time.Unix(1710688116, 0)
		expected := map[string]string{
			"SOAPACTION": action,
			"HNAP_AUTH":  "A00C07EC02491457ECDB82E012F2B083 1710688116",
		}

		headers := HNAPHeaders(action, "", "", now)
		assert.Equal(t, expected, headers)

		withPK := HNAPHeaders(action, "withoutloginkey", "", now)
		assert.Equal(t, withPK, headers)
	})
}

func TestNewLoginRequest(t *testing.T) {
	t.Run("returns new login request", func(t *testing.T) {
		action := "something awesome"
		user := "it's me"
		pass := "passw0rd!"

		request := NewLoginRequest(action, user, pass)
		assert.Equal(t, action, request.Login.Action)
		assert.Equal(t, user, request.Login.Username)
		assert.Equal(t, pass, request.Login.LoginPassword)
		assert.Equal(t, "LoginPassword", request.Login.PrivateLogin)
		assert.Empty(t, request.Login.Captcha)
	})
}

func TestNewChallenge(t *testing.T) {
	t.Run("returns new challenge from login response", func(t *testing.T) {
		publicKey := "public key"
		uid := "uid"
		message := "message"
		login := LoginResponse{
			LoginResponse: LoginResponseBody{
				Challenge:   message,
				Cookie:      uid,
				PublicKey:   publicKey,
				LoginResult: "not used",
			},
		}

		challenge := NewChallenge(login)
		assert.Equal(t, publicKey, challenge.PublicKey)
		assert.Equal(t, uid, challenge.UID)
		assert.Equal(t, message, challenge.Message)
	})
}

func TestNewCredentials(t *testing.T) {
	t.Run("returns encoded credentials", func(t *testing.T) {
		user := "it's me"
		uid := "uid"
		login := LoginResponse{
			LoginResponse: LoginResponseBody{
				Challenge:   "message",
				Cookie:      uid,
				PublicKey:   "public key",
				LoginResult: "not used",
			},
		}

		challenge := NewChallenge(login)
		c := NewCredentials(challenge, user, "passw0rd!")
		assert.Equal(t, uid, c.UID)
		assert.Equal(t, "F34A5F35BE08C01935A0BDB7D1F57B79", c.PrivateKey)
		assert.Equal(t, user, c.Username)
		assert.Equal(t, "0C41614BE33716EE09325AA96C434F05", c.Password)
	})
}

func TestCredentials_Empty(t *testing.T) {
	t.Run("returns true when uid empty", func(t *testing.T) {
		creds := Credentials{
			UID:        "",
			PrivateKey: "private key",
		}
		assert.True(t, creds.Empty())
	})

	t.Run("returns true when private key empty", func(t *testing.T) {
		creds := Credentials{
			UID:        "uid",
			PrivateKey: "",
		}
		assert.True(t, creds.Empty())
	})

	t.Run("returns true when uid and private key empty", func(t *testing.T) {
		creds := Credentials{
			UID:        "",
			PrivateKey: "",
		}
		assert.True(t, creds.Empty())
	})

	t.Run("returns false ", func(t *testing.T) {
		creds := Credentials{
			UID:        "uid",
			PrivateKey: "private key",
		}
		assert.False(t, creds.Empty())
	})
}
