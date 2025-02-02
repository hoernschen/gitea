// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"code.gitea.io/gitea/modules/log"

	"github.com/go-fed/httpsig"
)

// Federation settings
var (
	Federation = struct {
		Enabled             bool
		ShareUserStatistics bool
		MaxSize             int64
		Algorithms          []string
		DigestAlgorithm     string
		GetHeaders          []string
		PostHeaders         []string
	}{
		Enabled:             true,
		ShareUserStatistics: true,
		MaxSize:             4,
		Algorithms:          []string{"rsa-sha256", "rsa-sha512", "ed25519"},
		DigestAlgorithm:     "SHA-256",
		GetHeaders:          []string{"(request-target)", "Date"},
		PostHeaders:         []string{"(request-target)", "Date", "Digest"},
	}
)

// Constant slice of httpsig algorithm objects
var HttpsigAlgs []httpsig.Algorithm

func newFederationService() {
	if err := Cfg.Section("federation").MapTo(&Federation); err != nil {
		log.Fatal("Failed to map Federation settings: %v", err)
	} else if !httpsig.IsSupportedDigestAlgorithm(Federation.DigestAlgorithm) {
		log.Fatal("unsupported digest algorithm: %s", Federation.DigestAlgorithm)
		return
	}

	// Get MaxSize in bytes instead of MiB
	Federation.MaxSize = 1 << 20 * Federation.MaxSize

	HttpsigAlgs = make([]httpsig.Algorithm, len(Federation.Algorithms))
	for i, alg := range Federation.Algorithms {
		HttpsigAlgs[i] = httpsig.Algorithm(alg)
	}
}
