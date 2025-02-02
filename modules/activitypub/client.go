// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"strings"
	"time"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/proxy"
	"code.gitea.io/gitea/modules/setting"

	"github.com/go-fed/httpsig"
)

const (
	// ActivityStreamsContentType const
	ActivityStreamsContentType = `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`
	httpsigExpirationTime      = 60
)

// Gets the current time as an RFC 2616 formatted string
// RFC 2616 requires RFC 1123 dates but with GMT instead of UTC
func CurrentTime() string {
	return strings.ReplaceAll(time.Now().UTC().Format(time.RFC1123), "UTC", "GMT")
}

func containsRequiredHTTPHeaders(method string, headers []string) error {
	var hasRequestTarget, hasDate, hasDigest bool
	for _, header := range headers {
		hasRequestTarget = hasRequestTarget || header == httpsig.RequestTarget
		hasDate = hasDate || header == "Date"
		hasDigest = hasDigest || header == "Digest"
	}
	if !hasRequestTarget {
		return fmt.Errorf("missing http header for %s: %s", method, httpsig.RequestTarget)
	} else if !hasDate {
		return fmt.Errorf("missing http header for %s: Date", method)
	} else if !hasDigest && method != http.MethodGet {
		return fmt.Errorf("missing http header for %s: Digest", method)
	}
	return nil
}

// Client struct
type Client struct {
	client      *http.Client
	algs        []httpsig.Algorithm
	digestAlg   httpsig.DigestAlgorithm
	getHeaders  []string
	postHeaders []string
	priv        *rsa.PrivateKey
	pubID       string
}

// NewClient function
func NewClient(user *user_model.User, pubID string) (c *Client, err error) {
	if err = containsRequiredHTTPHeaders(http.MethodGet, setting.Federation.GetHeaders); err != nil {
		return
	} else if err = containsRequiredHTTPHeaders(http.MethodPost, setting.Federation.PostHeaders); err != nil {
		return
	}

	priv, err := GetPrivateKey(user)
	if err != nil {
		return
	}
	privPem, _ := pem.Decode([]byte(priv))
	privParsed, err := x509.ParsePKCS1PrivateKey(privPem.Bytes)
	if err != nil {
		return
	}

	c = &Client{
		client: &http.Client{
			Transport: &http.Transport{
				Proxy: proxy.Proxy(),
			},
		},
		algs:        setting.HttpsigAlgs,
		digestAlg:   httpsig.DigestAlgorithm(setting.Federation.DigestAlgorithm),
		getHeaders:  setting.Federation.GetHeaders,
		postHeaders: setting.Federation.PostHeaders,
		priv:        privParsed,
		pubID:       pubID,
	}
	return
}

// NewRequest function
func (c *Client) NewRequest(b []byte, to string) (req *http.Request, err error) {
	buf := bytes.NewBuffer(b)
	req, err = http.NewRequest(http.MethodPost, to, buf)
	if err != nil {
		return
	}
	req.Header.Add("Content-Type", ActivityStreamsContentType)
	req.Header.Add("Date", CurrentTime())
	req.Header.Add("User-Agent", "Gitea/"+setting.AppVer)
	signer, _, err := httpsig.NewSigner(c.algs, c.digestAlg, c.postHeaders, httpsig.Signature, httpsigExpirationTime)
	if err != nil {
		return
	}
	err = signer.SignRequest(c.priv, c.pubID, req, b)
	return
}

// Post function
func (c *Client) Post(b []byte, to string) (resp *http.Response, err error) {
	var req *http.Request
	if req, err = c.NewRequest(b, to); err != nil {
		return
	}
	resp, err = c.client.Do(req)
	return
}
