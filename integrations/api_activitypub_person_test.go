// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/activitypub"
	"code.gitea.io/gitea/modules/setting"

	ap "github.com/go-ap/activitypub"
	"github.com/stretchr/testify/assert"
)

func TestActivityPubPerson(t *testing.T) {
	onGiteaRun(t, func(*testing.T, *url.URL) {
		setting.Federation.Enabled = true
		defer func() {
			setting.Federation.Enabled = false
		}()

		username := "user2"
		req := NewRequestf(t, "GET", fmt.Sprintf("/api/v1/activitypub/user/%s", username))
		resp := MakeRequest(t, req, http.StatusOK)
		body := resp.Body.Bytes()
		assert.Contains(t, string(body), "@context")

		var person ap.Person
		err := person.UnmarshalJSON(body)
		assert.NoError(t, err)

		assert.Equal(t, ap.PersonType, person.Type)
		assert.Equal(t, username, person.PreferredUsername.String())
		keyID := person.GetID().String()
		assert.Regexp(t, fmt.Sprintf("activitypub/user/%s$", username), keyID)
		assert.Regexp(t, fmt.Sprintf("activitypub/user/%s/outbox$", username), person.Outbox.GetID().String())
		assert.Regexp(t, fmt.Sprintf("activitypub/user/%s/inbox$", username), person.Inbox.GetID().String())

		pubKey := person.PublicKey
		assert.NotNil(t, pubKey)
		publicKeyID := keyID + "#main-key"
		assert.Equal(t, pubKey.ID.String(), publicKeyID)

		pubKeyPem := pubKey.PublicKeyPem
		assert.NotNil(t, pubKeyPem)
		assert.Regexp(t, "^-----BEGIN PUBLIC KEY-----", pubKeyPem)
	})
}

func TestActivityPubMissingPerson(t *testing.T) {
	onGiteaRun(t, func(*testing.T, *url.URL) {
		setting.Federation.Enabled = true
		defer func() {
			setting.Federation.Enabled = false
		}()

		req := NewRequestf(t, "GET", "/api/v1/activitypub/user/nonexistentuser")
		resp := MakeRequest(t, req, http.StatusNotFound)
		assert.Contains(t, resp.Body.String(), "user redirect does not exist")
	})
}

func TestActivityPubPersonInbox(t *testing.T) {
	srv := httptest.NewServer(c)
	defer srv.Close()

	onGiteaRun(t, func(*testing.T, *url.URL) {
		appURL := setting.AppURL
		setting.Federation.Enabled = true
		setting.AppURL = srv.URL
		defer func() {
			setting.Federation.Enabled = false
			setting.Database.LogSQL = false
			setting.AppURL = appURL
		}()
		username1 := "user1"
		ctx := context.Background()
		user1, err := user_model.GetUserByName(ctx, username1)
		assert.NoError(t, err)
		user1url := fmt.Sprintf("%s/api/v1/activitypub/user/%s#main-key", srv.URL, username1)
		c, err := activitypub.NewClient(user1, user1url)
		assert.NoError(t, err)
		username2 := "user2"
		user2inboxurl := fmt.Sprintf("%s/api/v1/activitypub/user/%s/inbox", srv.URL, username2)

		// Signed request succeeds
		resp, err := c.Post([]byte{}, user2inboxurl)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)

		// Unsigned request fails
		req := NewRequest(t, "POST", user2inboxurl)
		MakeRequest(t, req, http.StatusInternalServerError)
	})
}
