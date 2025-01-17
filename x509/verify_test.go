// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x509

import (
	"encoding/pem"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dmin12/zcrypto1.3/x509/pkix"
)

type verifyTest struct {
	leaf          string
	intermediates []string
	roots         []string
	currentTime   int64
	dnsName       string
	keyUsages     []ExtKeyUsage

	errorCallback  func(*testing.T, int, error) bool
	expectedChains [][]string
	expiredChains  [][]string
}

var verifyTests = []verifyTest{
	{
		leaf:          googleLeaf,
		intermediates: []string{giag2Intermediate},
		roots:         []string{geoTrustRoot},
		currentTime:   1395785200,
		dnsName:       "www.google.com",

		expectedChains: [][]string{
			{"Google", "Google Internet Authority", "GeoTrust"},
		},
	},
	{
		leaf:          googleLeaf,
		intermediates: []string{giag2Intermediate},
		roots:         []string{geoTrustRoot},
		currentTime:   1395785200,
		dnsName:       "WwW.GooGLE.coM",

		expectedChains: [][]string{
			{"Google", "Google Internet Authority", "GeoTrust"},
		},
	},
	{
		leaf:          googleLeaf,
		intermediates: []string{giag2Intermediate},
		roots:         []string{geoTrustRoot},
		currentTime:   1,
		dnsName:       "WwW.GooGLE.coM",

		errorCallback: expectExpired,
	},
	{
		leaf:          googleLeaf,
		intermediates: []string{giag2Intermediate},
		roots:         []string{geoTrustRoot},
		currentTime:   1395785200,
		dnsName:       "www.example.com",

		expectedChains: [][]string{
			{"Google", "Google Internet Authority", "GeoTrust"},
		},
		errorCallback: expectHostnameError,
	},
	{
		leaf:          googleLeaf,
		intermediates: []string{giag2Intermediate},
		roots:         []string{geoTrustRoot},
		currentTime:   1,
		dnsName:       "www.example.com",

		errorCallback: expectExpired,
	},
	{
		leaf:          googleLeaf,
		roots:         []string{geoTrustRoot},
		currentTime:   1395785200,
		dnsName:       "www.google.com",
		errorCallback: expectAuthorityUnknown,
	},
	{
		leaf:          googleLeaf,
		intermediates: []string{geoTrustRoot, giag2Intermediate},
		roots:         []string{geoTrustRoot},
		currentTime:   1395785200,
		dnsName:       "www.google.com",

		expectedChains: [][]string{
			{"Google", "Google Internet Authority", "GeoTrust"},
		},
	},
	{
		leaf:          dnssecExpLeaf,
		intermediates: []string{startComIntermediate},
		roots:         []string{startComRoot},
		currentTime:   1302726541,

		expectedChains: [][]string{
			{"dnssec-exp", "StartCom Class 1", "StartCom Certification Authority"},
		},
	},
	{
		leaf:          dnssecExpLeaf,
		intermediates: []string{startComIntermediate, startComRoot},
		roots:         []string{startComRoot},
		currentTime:   1302726541,
		expectedChains: [][]string{
			{"dnssec-exp", "StartCom Class 1", "StartCom Certification Authority"},
		},
	},
	{
		leaf:          googleLeafWithInvalidHash,
		intermediates: []string{giag2Intermediate},
		roots:         []string{geoTrustRoot},
		currentTime:   1395785200,
		dnsName:       "www.google.com",
		errorCallback: expectHashError,
	},
	{
		// The default configuration should reject an S/MIME chain.
		leaf:          smimeLeaf,
		roots:         []string{smimeIntermediate},
		currentTime:   1339436154,
		errorCallback: expectUsageError,
	},
	{
		leaf:          smimeLeaf,
		roots:         []string{smimeIntermediate},
		currentTime:   1339436154,
		keyUsages:     []ExtKeyUsage{ExtKeyUsageServerAuth},
		errorCallback: expectUsageError,
	},
	{
		leaf:        smimeLeaf,
		roots:       []string{smimeIntermediate},
		currentTime: 1339436154,
		keyUsages:   []ExtKeyUsage{ExtKeyUsageEmailProtection},
		expectedChains: [][]string{
			{"Ryan Hurst", "GlobalSign PersonalSign 2 CA - G2"},
		},
	},
	{
		leaf:          megaLeaf,
		intermediates: []string{comodoIntermediate1},
		roots:         []string{comodoRoot},
		currentTime:   1360431182,
		expectedChains: [][]string{
			{"mega.co.nz", "EssentialSSL CA", "COMODO Certification Authority"},
		},
	},
	{
		// Check that a name constrained intermediate works even when
		// it lists multiple constraints.
		leaf:          nameConstraintsLeaf,
		intermediates: []string{nameConstraintsIntermediate1, nameConstraintsIntermediate2},
		roots:         []string{globalSignRoot},
		currentTime:   1382387896,
		dnsName:       "secure.iddl.vt.edu",

		expectedChains: [][]string{
			{
				"Technology-enhanced Learning and Online Strategies",
				"Virginia Tech Global Qualified Server CA",
				"Trusted Root CA G2",
				"GlobalSign Root CA",
			},
		},
	},
	{
		// Check that SHA-384 intermediates (which are popping up)
		// work.
		leaf:          moipLeafCert,
		intermediates: []string{comodoIntermediateSHA384, comodoRSAAuthority},
		roots:         []string{addTrustRoot},
		currentTime:   1397502195,
		dnsName:       "api.moip.com.br",

		expectedChains: [][]string{
			{
				"api.moip.com.br",
				"COMODO RSA Extended Validation Secure Server CA",
				"COMODO RSA Certification Authority",
				"AddTrust External CA Root",
			},
		},
	},
	{
		// Check the NotAfter < NotBefore is NeverValid
		leaf:          zcryptoNeverValid,
		intermediates: []string{zcryptoIntermediate},
		roots:         []string{zcryptoRoot},
		currentTime:   1622505600, // Tuesday 1st June 2021 12:00:00 AM
		dnsName:       "never-valid.example.com",

		errorCallback: expectNeverValid,
	},
	{
		leaf:          zcryptoValidBeforeIntermediate,
		intermediates: []string{zcryptoIntermediate},
		roots:         []string{zcryptoRoot},
		currentTime:   1527811200, // Friday 1st June 2018 12:00:00 AM
		dnsName:       "never-valid.example.com",

		errorCallback: expectNeverValid,
	},
	{
		leaf:          zcryptoRoot,
		intermediates: []string{zcryptoIntermediate},
		roots:         []string{zcryptoRoot},
		currentTime:   1527811200, // Friday 1st June 2018 12:00:00 AM,

		expectedChains: [][]string{
			{"ZCrypto Root Authority"},
		},
	},
	{
		leaf:          zcryptoRoot,
		intermediates: []string{zcryptoIntermediate},
		roots:         []string{zcryptoRoot},
		currentTime:   1830297600, // Saturday 1st Jan 2028 12:00:00 AM,

		expiredChains: [][]string{
			{"ZCrypto Root Authority"},
		},
		errorCallback: expectExpired,
	},
	{
		leaf:          zcryptoRoot,
		intermediates: []string{comodoIntermediateSHA384, comodoRSAAuthority},
		roots:         []string{addTrustRoot},
		currentTime:   1527811200, // Friday 1st June 2018 12:00:00 AM,

		errorCallback: expectCertificateInvalid(IsSelfSigned),
	},
	{
		leaf:          zcryptoRoot,
		intermediates: []string{zcryptoRoot, comodoRSAAuthority},
		roots:         []string{addTrustRoot},
		currentTime:   1527811200, // Friday 1st June 2018 12:00:00 AM,

		errorCallback: expectCertificateInvalid(IsSelfSigned),
	},
}

func expectHostnameError(t *testing.T, i int, err error) (ok bool) {
	if _, ok := err.(HostnameError); !ok {
		t.Errorf("#%d: error was not a HostnameError: %s", i, err)
		return false
	}
	return true
}

func expectExpired(t *testing.T, i int, err error) (ok bool) {
	if inval, ok := err.(CertificateInvalidError); !ok || inval.Reason != Expired {
		t.Errorf("#%d: error was not Expired: %s", i, err)
		return false
	}
	return true
}

func expectNeverValid(t *testing.T, i int, err error) (ok bool) {
	if inval, ok := err.(CertificateInvalidError); !ok || inval.Reason != NeverValid {
		t.Errorf("#%d: error was not NeverValid: %s", i, err)
		return false
	}
	return true
}

func expectUsageError(t *testing.T, i int, err error) (ok bool) {
	if inval, ok := err.(CertificateInvalidError); !ok || inval.Reason != IncompatibleUsage {
		t.Errorf("#%d: error was not IncompatibleUsage: %s", i, err)
		return false
	}
	return true
}

func expectAuthorityUnknown(t *testing.T, i int, err error) (ok bool) {
	if _, ok := err.(UnknownAuthorityError); !ok {
		t.Errorf("#%d: error was not UnknownAuthorityError: %s", i, err)
		return false
	}
	return true
}

func expectHashError(t *testing.T, i int, err error) bool {
	if err == nil {
		t.Errorf("#%d: no error resulted from invalid hash", i)
		return false
	}
	if expected := "algorithm unimplemented"; !strings.Contains(err.Error(), expected) {
		t.Errorf("#%d: error resulting from invalid hash didn't contain '%s', rather it was: %s", i, expected, err)
		return false
	}
	return true
}

func expectCertificateInvalid(reason InvalidReason) func(*testing.T, int, error) bool {
	expectedReason := reason
	return func(t *testing.T, i int, err error) bool {
		if err == nil {
			t.Errorf("#%d: no error when expectin CertificateInvalidError", i)
			return false
		}
		invalidError, ok := err.(CertificateInvalidError)
		if !ok {
			t.Errorf("#%d: got an error that wasn't CertificateInvalidError", i)
			return false
		}
		if invalidError.Reason != expectedReason {
			t.Errorf("#%d: got invalid reason %v, expected %v", i, invalidError.Reason, expectedReason)
			return false
		}
		return true
	}
}

func certificateFromPEM(pemBytes string) (*Certificate, error) {
	block, _ := pem.Decode([]byte(pemBytes))
	if block == nil {
		return nil, errors.New("failed to decode PEM")
	}
	return ParseCertificate(block.Bytes)
}

func testVerify(t *testing.T) {
	for i, test := range verifyTests {
		opts := VerifyOptions{
			Intermediates: NewCertPool(),
			DNSName:       test.dnsName,
			CurrentTime:   time.Unix(test.currentTime, 0),
			KeyUsages:     test.keyUsages,
		}

		opts.Roots = NewCertPool()
		for j, root := range test.roots {
			ok := opts.Roots.AppendCertsFromPEM([]byte(root))
			if !ok {
				t.Errorf("#%d: failed to parse root #%d", i, j)
				return
			}
		}

		for j, intermediate := range test.intermediates {
			ok := opts.Intermediates.AppendCertsFromPEM([]byte(intermediate))
			if !ok {
				t.Errorf("#%d: failed to parse intermediate #%d", i, j)
				return
			}
		}

		leaf, err := certificateFromPEM(test.leaf)
		if err != nil {
			t.Errorf("#%d: failed to parse leaf: %s", i, err)
			return
		}

		chains, expiredChains, _, err := leaf.Verify(opts)

		if test.errorCallback == nil && err != nil {
			t.Errorf("#%d: unexpected error: %s", i, err)
		}
		if test.errorCallback != nil {
			if !test.errorCallback(t, i, err) {
				return
			}
		}

		if len(chains) != len(test.expectedChains) {
			t.Errorf("#%d: wanted %d chains, got %d", i, len(test.expectedChains), len(chains))
		}

		if test.expiredChains != nil && len(expiredChains) != len(test.expiredChains) {
			t.Errorf("#%d: wanted %d expired chains, got %d", i, len(test.expiredChains), len(expiredChains))
		}

		// We check that each returned chain matches a chain from
		// expectedChains but an entry in expectedChains can't match
		// two chains.
		seenChains := make([]bool, len(chains))
	NextOutputChain:
		for _, chain := range chains {
		TryNextExpected:
			for j, expectedChain := range test.expectedChains {
				if seenChains[j] {
					continue
				}
				if len(chain) != len(expectedChain) {
					continue
				}
				for k, cert := range chain {
					if strings.Index(nameToKey(&cert.Subject), expectedChain[k]) == -1 {
						continue TryNextExpected
					}
				}
				// we matched
				seenChains[j] = true
				continue NextOutputChain
			}
			t.Errorf("#%d: No expected chain matched %s", i, chainToDebugString(chain))
		}
	}
}

func TestGoVerify(t *testing.T) {
	testVerify(t)
}

func chainToDebugString(chain []*Certificate) string {
	var chainStr string
	for _, cert := range chain {
		if len(chainStr) > 0 {
			chainStr += " -> "
		}
		chainStr += nameToKey(&cert.Subject)
	}
	return chainStr
}

func nameToKey(name *pkix.Name) string {
	return strings.Join(name.Country, ",") + "/" + strings.Join(name.Organization, ",") + "/" + strings.Join(name.OrganizationalUnit, ",") + "/" + name.CommonName
}

const geoTrustRoot = `-----BEGIN CERTIFICATE-----
MIIDVDCCAjygAwIBAgIDAjRWMA0GCSqGSIb3DQEBBQUAMEIxCzAJBgNVBAYTAlVT
MRYwFAYDVQQKEw1HZW9UcnVzdCBJbmMuMRswGQYDVQQDExJHZW9UcnVzdCBHbG9i
YWwgQ0EwHhcNMDIwNTIxMDQwMDAwWhcNMjIwNTIxMDQwMDAwWjBCMQswCQYDVQQG
EwJVUzEWMBQGA1UEChMNR2VvVHJ1c3QgSW5jLjEbMBkGA1UEAxMSR2VvVHJ1c3Qg
R2xvYmFsIENBMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA2swYYzD9
9BcjGlZ+W988bDjkcbd4kdS8odhM+KhDtgPpTSEHCIjaWC9mOSm9BXiLnTjoBbdq
fnGk5sRgprDvgOSJKA+eJdbtg/OtppHHmMlCGDUUna2YRpIuT8rxh0PBFpVXLVDv
iS2Aelet8u5fa9IAjbkU+BQVNdnARqN7csiRv8lVK83Qlz6cJmTM386DGXHKTubU
1XupGc1V3sjs0l44U+VcT4wt/lAjNvxm5suOpDkZALeVAjmRCw7+OC7RHQWa9k0+
bw8HHa8sHo9gOeL6NlMTOdReJivbPagUvTLrGAMoUgRx5aszPeE4uwc2hGKceeoW
MPRfwCvocWvk+QIDAQABo1MwUTAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBTA
ephojYn7qwVkDBF9qn1luMrMTjAfBgNVHSMEGDAWgBTAephojYn7qwVkDBF9qn1l
uMrMTjANBgkqhkiG9w0BAQUFAAOCAQEANeMpauUvXVSOKVCUn5kaFOSPeCpilKIn
Z57QzxpeR+nBsqTP3UEaBU6bS+5Kb1VSsyShNwrrZHYqLizz/Tt1kL/6cdjHPTfS
tQWVYrmm3ok9Nns4d0iXrKYgjy6myQzCsplFAMfOEVEiIuCl6rYVSAlk6l5PdPcF
PseKUgzbFbS9bZvlxrFUaKnjaZC2mqUPuLk/IH2uSrW4nOQdtqvmlKXBx4Ot2/Un
hw4EbNX/3aBd7YdStysVAq45pmp06drE57xNNB6pXE0zX5IJL4hmXXeXxx12E6nV
5fEWCRE11azbJHFwLJhWC9kXtNHjUStedejV0NxPNO3CBWaAocvmMw==
-----END CERTIFICATE-----
`

const giag2Intermediate = `-----BEGIN CERTIFICATE-----
MIIEBDCCAuygAwIBAgIDAjppMA0GCSqGSIb3DQEBBQUAMEIxCzAJBgNVBAYTAlVT
MRYwFAYDVQQKEw1HZW9UcnVzdCBJbmMuMRswGQYDVQQDExJHZW9UcnVzdCBHbG9i
YWwgQ0EwHhcNMTMwNDA1MTUxNTU1WhcNMTUwNDA0MTUxNTU1WjBJMQswCQYDVQQG
EwJVUzETMBEGA1UEChMKR29vZ2xlIEluYzElMCMGA1UEAxMcR29vZ2xlIEludGVy
bmV0IEF1dGhvcml0eSBHMjCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEB
AJwqBHdc2FCROgajguDYUEi8iT/xGXAaiEZ+4I/F8YnOIe5a/mENtzJEiaB0C1NP
VaTOgmKV7utZX8bhBYASxF6UP7xbSDj0U/ck5vuR6RXEz/RTDfRK/J9U3n2+oGtv
h8DQUB8oMANA2ghzUWx//zo8pzcGjr1LEQTrfSTe5vn8MXH7lNVg8y5Kr0LSy+rE
ahqyzFPdFUuLH8gZYR/Nnag+YyuENWllhMgZxUYi+FOVvuOAShDGKuy6lyARxzmZ
EASg8GF6lSWMTlJ14rbtCMoU/M4iarNOz0YDl5cDfsCx3nuvRTPPuj5xt970JSXC
DTWJnZ37DhF5iR43xa+OcmkCAwEAAaOB+zCB+DAfBgNVHSMEGDAWgBTAephojYn7
qwVkDBF9qn1luMrMTjAdBgNVHQ4EFgQUSt0GFhu89mi1dvWBtrtiGrpagS8wEgYD
VR0TAQH/BAgwBgEB/wIBADAOBgNVHQ8BAf8EBAMCAQYwOgYDVR0fBDMwMTAvoC2g
K4YpaHR0cDovL2NybC5nZW90cnVzdC5jb20vY3Jscy9ndGdsb2JhbC5jcmwwPQYI
KwYBBQUHAQEEMTAvMC0GCCsGAQUFBzABhiFodHRwOi8vZ3RnbG9iYWwtb2NzcC5n
ZW90cnVzdC5jb20wFwYDVR0gBBAwDjAMBgorBgEEAdZ5AgUBMA0GCSqGSIb3DQEB
BQUAA4IBAQA21waAESetKhSbOHezI6B1WLuxfoNCunLaHtiONgaX4PCVOzf9G0JY
/iLIa704XtE7JW4S615ndkZAkNoUyHgN7ZVm2o6Gb4ChulYylYbc3GrKBIxbf/a/
zG+FA1jDaFETzf3I93k9mTXwVqO94FntT0QJo544evZG0R0SnU++0ED8Vf4GXjza
HFa9llF7b1cq26KqltyMdMKVvvBulRP/F/A8rLIQjcxz++iPAsbw+zOzlTvjwsto
WHPbqCRiOwY1nQ2pM714A5AuTHhdUDqB1O6gyHA43LL5Z/qHQF1hwFGPa4NrzQU6
yuGnBXj8ytqU0CwIPX4WecigUCAkVDNx
-----END CERTIFICATE-----
`

const googleLeaf = `-----BEGIN CERTIFICATE-----
MIIEdjCCA16gAwIBAgIIcR5k4dkoe04wDQYJKoZIhvcNAQEFBQAwSTELMAkGA1UE
BhMCVVMxEzARBgNVBAoTCkdvb2dsZSBJbmMxJTAjBgNVBAMTHEdvb2dsZSBJbnRl
cm5ldCBBdXRob3JpdHkgRzIwHhcNMTQwMzEyMDkzODMwWhcNMTQwNjEwMDAwMDAw
WjBoMQswCQYDVQQGEwJVUzETMBEGA1UECAwKQ2FsaWZvcm5pYTEWMBQGA1UEBwwN
TW91bnRhaW4gVmlldzETMBEGA1UECgwKR29vZ2xlIEluYzEXMBUGA1UEAwwOd3d3
Lmdvb2dsZS5jb20wggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQC4zYCe
m0oUBhwE0EwBr65eBOcgcQO2PaSIAB2dEP/c1EMX2tOy0ov8rk83ePhJ+MWdT1z6
jge9X4zQQI8ZyA9qIiwrKBZOi8DNUvrqNZC7fJAVRrb9aX/99uYOJCypIbpmWG1q
fhbHjJewhwf8xYPj71eU4rLG80a+DapWmphtfq3h52lDQIBzLVf1yYbyrTaELaz4
NXF7HXb5YkId/gxIsSzM0aFUVu2o8sJcLYAsJqwfFKBKOMxUcn545nlspf0mTcWZ
0APlbwsKznNs4/xCDwIxxWjjqgHrYAFl6y07i1gzbAOqdNEyR24p+3JWI8WZBlBI
dk2KGj0W1fIfsvyxAgMBAAGjggFBMIIBPTAdBgNVHSUEFjAUBggrBgEFBQcDAQYI
KwYBBQUHAwIwGQYDVR0RBBIwEIIOd3d3Lmdvb2dsZS5jb20waAYIKwYBBQUHAQEE
XDBaMCsGCCsGAQUFBzAChh9odHRwOi8vcGtpLmdvb2dsZS5jb20vR0lBRzIuY3J0
MCsGCCsGAQUFBzABhh9odHRwOi8vY2xpZW50czEuZ29vZ2xlLmNvbS9vY3NwMB0G
A1UdDgQWBBTXD5Bx6iqT+dmEhbFL4OUoHyZn8zAMBgNVHRMBAf8EAjAAMB8GA1Ud
IwQYMBaAFErdBhYbvPZotXb1gba7Yhq6WoEvMBcGA1UdIAQQMA4wDAYKKwYBBAHW
eQIFATAwBgNVHR8EKTAnMCWgI6Ahhh9odHRwOi8vcGtpLmdvb2dsZS5jb20vR0lB
RzIuY3JsMA0GCSqGSIb3DQEBBQUAA4IBAQCR3RJtHzgDh33b/MI1ugiki+nl8Ikj
5larbJRE/rcA5oite+QJyAr6SU1gJJ/rRrK3ItVEHr9L621BCM7GSdoNMjB9MMcf
tJAW0kYGJ+wqKm53wG/JaOADTnnq2Mt/j6F2uvjgN/ouns1nRHufIvd370N0LeH+
orKqTuAPzXK7imQk6+OycYABbqCtC/9qmwRd8wwn7sF97DtYfK8WuNHtFalCAwyi
8LxJJYJCLWoMhZ+V8GZm+FOex5qkQAjnZrtNlbQJ8ro4r+rpKXtmMFFhfa+7L+PA
Kom08eUK8skxAzfDDijZPh10VtJ66uBoiDPdT+uCBehcBIcmSTrKjFGX
-----END CERTIFICATE-----
`

// googleLeafWithInvalidHash is the same as googleLeaf, but the signature
// algorithm in the certificate contains a nonsense OID.
const googleLeafWithInvalidHash = `-----BEGIN CERTIFICATE-----
MIIEdjCCA16gAwIBAgIIcR5k4dkoe04wDQYJKoZIhvcNAWAFBQAwSTELMAkGA1UE
BhMCVVMxEzARBgNVBAoTCkdvb2dsZSBJbmMxJTAjBgNVBAMTHEdvb2dsZSBJbnRl
cm5ldCBBdXRob3JpdHkgRzIwHhcNMTQwMzEyMDkzODMwWhcNMTQwNjEwMDAwMDAw
WjBoMQswCQYDVQQGEwJVUzETMBEGA1UECAwKQ2FsaWZvcm5pYTEWMBQGA1UEBwwN
TW91bnRhaW4gVmlldzETMBEGA1UECgwKR29vZ2xlIEluYzEXMBUGA1UEAwwOd3d3
Lmdvb2dsZS5jb20wggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQC4zYCe
m0oUBhwE0EwBr65eBOcgcQO2PaSIAB2dEP/c1EMX2tOy0ov8rk83ePhJ+MWdT1z6
jge9X4zQQI8ZyA9qIiwrKBZOi8DNUvrqNZC7fJAVRrb9aX/99uYOJCypIbpmWG1q
fhbHjJewhwf8xYPj71eU4rLG80a+DapWmphtfq3h52lDQIBzLVf1yYbyrTaELaz4
NXF7HXb5YkId/gxIsSzM0aFUVu2o8sJcLYAsJqwfFKBKOMxUcn545nlspf0mTcWZ
0APlbwsKznNs4/xCDwIxxWjjqgHrYAFl6y07i1gzbAOqdNEyR24p+3JWI8WZBlBI
dk2KGj0W1fIfsvyxAgMBAAGjggFBMIIBPTAdBgNVHSUEFjAUBggrBgEFBQcDAQYI
KwYBBQUHAwIwGQYDVR0RBBIwEIIOd3d3Lmdvb2dsZS5jb20waAYIKwYBBQUHAQEE
XDBaMCsGCCsGAQUFBzAChh9odHRwOi8vcGtpLmdvb2dsZS5jb20vR0lBRzIuY3J0
MCsGCCsGAQUFBzABhh9odHRwOi8vY2xpZW50czEuZ29vZ2xlLmNvbS9vY3NwMB0G
A1UdDgQWBBTXD5Bx6iqT+dmEhbFL4OUoHyZn8zAMBgNVHRMBAf8EAjAAMB8GA1Ud
IwQYMBaAFErdBhYbvPZotXb1gba7Yhq6WoEvMBcGA1UdIAQQMA4wDAYKKwYBBAHW
eQIFATAwBgNVHR8EKTAnMCWgI6Ahhh9odHRwOi8vcGtpLmdvb2dsZS5jb20vR0lB
RzIuY3JsMA0GCSqGSIb3DQFgBQUAA4IBAQCR3RJtHzgDh33b/MI1ugiki+nl8Ikj
5larbJRE/rcA5oite+QJyAr6SU1gJJ/rRrK3ItVEHr9L621BCM7GSdoNMjB9MMcf
tJAW0kYGJ+wqKm53wG/JaOADTnnq2Mt/j6F2uvjgN/ouns1nRHufIvd370N0LeH+
orKqTuAPzXK7imQk6+OycYABbqCtC/9qmwRd8wwn7sF97DtYfK8WuNHtFalCAwyi
8LxJJYJCLWoMhZ+V8GZm+FOex5qkQAjnZrtNlbQJ8ro4r+rpKXtmMFFhfa+7L+PA
Kom08eUK8skxAzfDDijZPh10VtJ66uBoiDPdT+uCBehcBIcmSTrKjFGX
-----END CERTIFICATE-----
`

const dnssecExpLeaf = `-----BEGIN CERTIFICATE-----
MIIGzTCCBbWgAwIBAgIDAdD6MA0GCSqGSIb3DQEBBQUAMIGMMQswCQYDVQQGEwJJ
TDEWMBQGA1UEChMNU3RhcnRDb20gTHRkLjErMCkGA1UECxMiU2VjdXJlIERpZ2l0
YWwgQ2VydGlmaWNhdGUgU2lnbmluZzE4MDYGA1UEAxMvU3RhcnRDb20gQ2xhc3Mg
MSBQcmltYXJ5IEludGVybWVkaWF0ZSBTZXJ2ZXIgQ0EwHhcNMTAwNzA0MTQ1MjQ1
WhcNMTEwNzA1MTA1NzA0WjCBwTEgMB4GA1UEDRMXMjIxMTM3LWxpOWE5dHhJRzZM
NnNyVFMxCzAJBgNVBAYTAlVTMR4wHAYDVQQKExVQZXJzb25hIE5vdCBWYWxpZGF0
ZWQxKTAnBgNVBAsTIFN0YXJ0Q29tIEZyZWUgQ2VydGlmaWNhdGUgTWVtYmVyMRsw
GQYDVQQDExJ3d3cuZG5zc2VjLWV4cC5vcmcxKDAmBgkqhkiG9w0BCQEWGWhvc3Rt
YXN0ZXJAZG5zc2VjLWV4cC5vcmcwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEK
AoIBAQDEdF/22vaxrPbqpgVYMWi+alfpzBctpbfLBdPGuqOazJdCT0NbWcK8/+B4
X6OlSOURNIlwLzhkmwVsWdVv6dVSaN7d4yI/fJkvgfDB9+au+iBJb6Pcz8ULBfe6
D8HVvqKdORp6INzHz71z0sghxrQ0EAEkoWAZLh+kcn2ZHdcmZaBNUfjmGbyU6PRt
RjdqoP+owIaC1aktBN7zl4uO7cRjlYFdusINrh2kPP02KAx2W84xjxX1uyj6oS6e
7eBfvcwe8czW/N1rbE0CoR7h9+HnIrjnVG9RhBiZEiw3mUmF++Up26+4KTdRKbu3
+BL4yMpfd66z0+zzqu+HkvyLpFn5AgMBAAGjggL/MIIC+zAJBgNVHRMEAjAAMAsG
A1UdDwQEAwIDqDATBgNVHSUEDDAKBggrBgEFBQcDATAdBgNVHQ4EFgQUy04I5guM
drzfh2JQaXhgV86+4jUwHwYDVR0jBBgwFoAU60I00Jiwq5/0G2sI98xkLu8OLEUw
LQYDVR0RBCYwJIISd3d3LmRuc3NlYy1leHAub3Jngg5kbnNzZWMtZXhwLm9yZzCC
AUIGA1UdIASCATkwggE1MIIBMQYLKwYBBAGBtTcBAgIwggEgMC4GCCsGAQUFBwIB
FiJodHRwOi8vd3d3LnN0YXJ0c3NsLmNvbS9wb2xpY3kucGRmMDQGCCsGAQUFBwIB
FihodHRwOi8vd3d3LnN0YXJ0c3NsLmNvbS9pbnRlcm1lZGlhdGUucGRmMIG3Bggr
BgEFBQcCAjCBqjAUFg1TdGFydENvbSBMdGQuMAMCAQEagZFMaW1pdGVkIExpYWJp
bGl0eSwgc2VlIHNlY3Rpb24gKkxlZ2FsIExpbWl0YXRpb25zKiBvZiB0aGUgU3Rh
cnRDb20gQ2VydGlmaWNhdGlvbiBBdXRob3JpdHkgUG9saWN5IGF2YWlsYWJsZSBh
dCBodHRwOi8vd3d3LnN0YXJ0c3NsLmNvbS9wb2xpY3kucGRmMGEGA1UdHwRaMFgw
KqAooCaGJGh0dHA6Ly93d3cuc3RhcnRzc2wuY29tL2NydDEtY3JsLmNybDAqoCig
JoYkaHR0cDovL2NybC5zdGFydHNzbC5jb20vY3J0MS1jcmwuY3JsMIGOBggrBgEF
BQcBAQSBgTB/MDkGCCsGAQUFBzABhi1odHRwOi8vb2NzcC5zdGFydHNzbC5jb20v
c3ViL2NsYXNzMS9zZXJ2ZXIvY2EwQgYIKwYBBQUHMAKGNmh0dHA6Ly93d3cuc3Rh
cnRzc2wuY29tL2NlcnRzL3N1Yi5jbGFzczEuc2VydmVyLmNhLmNydDAjBgNVHRIE
HDAahhhodHRwOi8vd3d3LnN0YXJ0c3NsLmNvbS8wDQYJKoZIhvcNAQEFBQADggEB
ACXj6SB59KRJPenn6gUdGEqcta97U769SATyiQ87i9er64qLwvIGLMa3o2Rcgl2Y
kghUeyLdN/EXyFBYA8L8uvZREPoc7EZukpT/ZDLXy9i2S0jkOxvF2fD/XLbcjGjM
iEYG1/6ASw0ri9C0k4oDDoJLCoeH9++yqF7SFCCMcDkJqiAGXNb4euDpa8vCCtEQ
CSS+ObZbfkreRt3cNCf5LfCXe9OsTnCfc8Cuq81c0oLaG+SmaLUQNBuToq8e9/Zm
+b+/a3RVjxmkV5OCcGVBxsXNDn54Q6wsdw0TBMcjwoEndzpLS7yWgFbbkq5ZiGpw
Qibb2+CfKuQ+WFV1GkVQmVA=
-----END CERTIFICATE-----`

const startComIntermediate = `-----BEGIN CERTIFICATE-----
MIIGNDCCBBygAwIBAgIBGDANBgkqhkiG9w0BAQUFADB9MQswCQYDVQQGEwJJTDEW
MBQGA1UEChMNU3RhcnRDb20gTHRkLjErMCkGA1UECxMiU2VjdXJlIERpZ2l0YWwg
Q2VydGlmaWNhdGUgU2lnbmluZzEpMCcGA1UEAxMgU3RhcnRDb20gQ2VydGlmaWNh
dGlvbiBBdXRob3JpdHkwHhcNMDcxMDI0MjA1NDE3WhcNMTcxMDI0MjA1NDE3WjCB
jDELMAkGA1UEBhMCSUwxFjAUBgNVBAoTDVN0YXJ0Q29tIEx0ZC4xKzApBgNVBAsT
IlNlY3VyZSBEaWdpdGFsIENlcnRpZmljYXRlIFNpZ25pbmcxODA2BgNVBAMTL1N0
YXJ0Q29tIENsYXNzIDEgUHJpbWFyeSBJbnRlcm1lZGlhdGUgU2VydmVyIENBMIIB
IjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAtonGrO8JUngHrJJj0PREGBiE
gFYfka7hh/oyULTTRwbw5gdfcA4Q9x3AzhA2NIVaD5Ksg8asWFI/ujjo/OenJOJA
pgh2wJJuniptTT9uYSAK21ne0n1jsz5G/vohURjXzTCm7QduO3CHtPn66+6CPAVv
kvek3AowHpNz/gfK11+AnSJYUq4G2ouHI2mw5CrY6oPSvfNx23BaKA+vWjhwRRI/
ME3NO68X5Q/LoKldSKqxYVDLNM08XMML6BDAjJvwAwNi/rJsPnIO7hxDKslIDlc5
xDEhyBDBLIf+VJVSH1I8MRKbf+fAoKVZ1eKPPvDVqOHXcDGpxLPPr21TLwb0pwID
AQABo4IBrTCCAakwDwYDVR0TAQH/BAUwAwEB/zAOBgNVHQ8BAf8EBAMCAQYwHQYD
VR0OBBYEFOtCNNCYsKuf9BtrCPfMZC7vDixFMB8GA1UdIwQYMBaAFE4L7xqkQFul
F2mHMMo0aEPQQa7yMGYGCCsGAQUFBwEBBFowWDAnBggrBgEFBQcwAYYbaHR0cDov
L29jc3Auc3RhcnRzc2wuY29tL2NhMC0GCCsGAQUFBzAChiFodHRwOi8vd3d3LnN0
YXJ0c3NsLmNvbS9zZnNjYS5jcnQwWwYDVR0fBFQwUjAnoCWgI4YhaHR0cDovL3d3
dy5zdGFydHNzbC5jb20vc2ZzY2EuY3JsMCegJaAjhiFodHRwOi8vY3JsLnN0YXJ0
c3NsLmNvbS9zZnNjYS5jcmwwgYAGA1UdIAR5MHcwdQYLKwYBBAGBtTcBAgEwZjAu
BggrBgEFBQcCARYiaHR0cDovL3d3dy5zdGFydHNzbC5jb20vcG9saWN5LnBkZjA0
BggrBgEFBQcCARYoaHR0cDovL3d3dy5zdGFydHNzbC5jb20vaW50ZXJtZWRpYXRl
LnBkZjANBgkqhkiG9w0BAQUFAAOCAgEAIQlJPqWIbuALi0jaMU2P91ZXouHTYlfp
tVbzhUV1O+VQHwSL5qBaPucAroXQ+/8gA2TLrQLhxpFy+KNN1t7ozD+hiqLjfDen
xk+PNdb01m4Ge90h2c9W/8swIkn+iQTzheWq8ecf6HWQTd35RvdCNPdFWAwRDYSw
xtpdPvkBnufh2lWVvnQce/xNFE+sflVHfXv0pQ1JHpXo9xLBzP92piVH0PN1Nb6X
t1gW66pceG/sUzCv6gRNzKkC4/C2BBL2MLERPZBOVmTX3DxDX3M570uvh+v2/miI
RHLq0gfGabDBoYvvF0nXYbFFSF87ICHpW7LM9NfpMfULFWE7epTj69m8f5SuauNi
YpaoZHy4h/OZMn6SolK+u/hlz8nyMPyLwcKmltdfieFcNID1j0cHL7SRv7Gifl9L
WtBbnySGBVFaaQNlQ0lxxeBvlDRr9hvYqbBMflPrj0jfyjO1SPo2ShpTpjMM0InN
SRXNiTE8kMBy12VLUjWKRhFEuT2OKGWmPnmeXAhEKa2wNREuIU640ucQPl2Eg7PD
wuTSxv0JS3QJ3fGz0xk+gA2iCxnwOOfFwq/iI9th4p1cbiCJSS4jarJiwUW0n6+L
p/EiO/h94pDQehn7Skzj0n1fSoMD7SfWI55rjbRZotnvbIIp3XUZPD9MEI3vu3Un
0q6Dp6jOW6c=
-----END CERTIFICATE-----`

const startComRoot = `-----BEGIN CERTIFICATE-----
MIIHyTCCBbGgAwIBAgIBATANBgkqhkiG9w0BAQUFADB9MQswCQYDVQQGEwJJTDEW
MBQGA1UEChMNU3RhcnRDb20gTHRkLjErMCkGA1UECxMiU2VjdXJlIERpZ2l0YWwg
Q2VydGlmaWNhdGUgU2lnbmluZzEpMCcGA1UEAxMgU3RhcnRDb20gQ2VydGlmaWNh
dGlvbiBBdXRob3JpdHkwHhcNMDYwOTE3MTk0NjM2WhcNMzYwOTE3MTk0NjM2WjB9
MQswCQYDVQQGEwJJTDEWMBQGA1UEChMNU3RhcnRDb20gTHRkLjErMCkGA1UECxMi
U2VjdXJlIERpZ2l0YWwgQ2VydGlmaWNhdGUgU2lnbmluZzEpMCcGA1UEAxMgU3Rh
cnRDb20gQ2VydGlmaWNhdGlvbiBBdXRob3JpdHkwggIiMA0GCSqGSIb3DQEBAQUA
A4ICDwAwggIKAoICAQDBiNsJvGxGfHiflXu1M5DycmLWwTYgIiRezul38kMKogZk
pMyONvg45iPwbm2xPN1yo4UcodM9tDMr0y+v/uqwQVlntsQGfQqedIXWeUyAN3rf
OQVSWff0G0ZDpNKFhdLDcfN1YjS6LIp/Ho/u7TTQEceWzVI9ujPW3U3eCztKS5/C
Ji/6tRYccjV3yjxd5srhJosaNnZcAdt0FCX+7bWgiA/deMotHweXMAEtcnn6RtYT
Kqi5pquDSR3l8u/d5AGOGAqPY1MWhWKpDhk6zLVmpsJrdAfkK+F2PrRt2PZE4XNi
HzvEvqBTViVsUQn3qqvKv3b9bZvzndu/PWa8DFaqr5hIlTpL36dYUNk4dalb6kMM
Av+Z6+hsTXBbKWWc3apdzK8BMewM69KN6Oqce+Zu9ydmDBpI125C4z/eIT574Q1w
+2OqqGwaVLRcJXrJosmLFqa7LH4XXgVNWG4SHQHuEhANxjJ/GP/89PrNbpHoNkm+
Gkhpi8KWTRoSsmkXwQqQ1vp5Iki/untp+HDH+no32NgN0nZPV/+Qt+OR0t3vwmC3
Zzrd/qqc8NSLf3Iizsafl7b4r4qgEKjZ+xjGtrVcUjyJthkqcwEKDwOzEmDyei+B
26Nu/yYwl/WL3YlXtq09s68rxbd2AvCl1iuahhQqcvbjM4xdCUsT37uMdBNSSwID
AQABo4ICUjCCAk4wDAYDVR0TBAUwAwEB/zALBgNVHQ8EBAMCAa4wHQYDVR0OBBYE
FE4L7xqkQFulF2mHMMo0aEPQQa7yMGQGA1UdHwRdMFswLKAqoCiGJmh0dHA6Ly9j
ZXJ0LnN0YXJ0Y29tLm9yZy9zZnNjYS1jcmwuY3JsMCugKaAnhiVodHRwOi8vY3Js
LnN0YXJ0Y29tLm9yZy9zZnNjYS1jcmwuY3JsMIIBXQYDVR0gBIIBVDCCAVAwggFM
BgsrBgEEAYG1NwEBATCCATswLwYIKwYBBQUHAgEWI2h0dHA6Ly9jZXJ0LnN0YXJ0
Y29tLm9yZy9wb2xpY3kucGRmMDUGCCsGAQUFBwIBFilodHRwOi8vY2VydC5zdGFy
dGNvbS5vcmcvaW50ZXJtZWRpYXRlLnBkZjCB0AYIKwYBBQUHAgIwgcMwJxYgU3Rh
cnQgQ29tbWVyY2lhbCAoU3RhcnRDb20pIEx0ZC4wAwIBARqBl0xpbWl0ZWQgTGlh
YmlsaXR5LCByZWFkIHRoZSBzZWN0aW9uICpMZWdhbCBMaW1pdGF0aW9ucyogb2Yg
dGhlIFN0YXJ0Q29tIENlcnRpZmljYXRpb24gQXV0aG9yaXR5IFBvbGljeSBhdmFp
bGFibGUgYXQgaHR0cDovL2NlcnQuc3RhcnRjb20ub3JnL3BvbGljeS5wZGYwEQYJ
YIZIAYb4QgEBBAQDAgAHMDgGCWCGSAGG+EIBDQQrFilTdGFydENvbSBGcmVlIFNT
TCBDZXJ0aWZpY2F0aW9uIEF1dGhvcml0eTANBgkqhkiG9w0BAQUFAAOCAgEAFmyZ
9GYMNPXQhV59CuzaEE44HF7fpiUFS5Eyweg78T3dRAlbB0mKKctmArexmvclmAk8
jhvh3TaHK0u7aNM5Zj2gJsfyOZEdUauCe37Vzlrk4gNXcGmXCPleWKYK34wGmkUW
FjgKXlf2Ysd6AgXmvB618p70qSmD+LIU424oh0TDkBreOKk8rENNZEXO3SipXPJz
ewT4F+irsfMuXGRuczE6Eri8sxHkfY+BUZo7jYn0TZNmezwD7dOaHZrzZVD1oNB1
ny+v8OqCQ5j4aZyJecRDjkZy42Q2Eq/3JR44iZB3fsNrarnDy0RLrHiQi+fHLB5L
EUTINFInzQpdn4XBidUaePKVEFMy3YCEZnXZtWgo+2EuvoSoOMCZEoalHmdkrQYu
L6lwhceWD3yJZfWOQ1QOq92lgDmUYMA0yZZwLKMS9R9Ie70cfmu3nZD0Ijuu+Pwq
yvqCUqDvr0tVk+vBtfAii6w0TiYiBKGHLHVKt+V9E9e4DGTANtLJL4YSjCMJwRuC
O3NJo2pXh5Tl1njFmUNj403gdy3hZZlyaQQaRwnmDwFWJPsfvw55qVguucQJAX6V
um0ABj6y6koQOdjQK/W/7HW/lwLFCRsI3FU34oH7N4RDYiDK51ZLZer+bMEkkySh
NOsF/5oirpt9P/FlUQqmMGqz9IgcgA38corog14=
-----END CERTIFICATE-----`

const startComRootSHA256 = `-----BEGIN CERTIFICATE-----
MIIHhzCCBW+gAwIBAgIBLTANBgkqhkiG9w0BAQsFADB9MQswCQYDVQQGEwJJTDEW
MBQGA1UEChMNU3RhcnRDb20gTHRkLjErMCkGA1UECxMiU2VjdXJlIERpZ2l0YWwg
Q2VydGlmaWNhdGUgU2lnbmluZzEpMCcGA1UEAxMgU3RhcnRDb20gQ2VydGlmaWNh
dGlvbiBBdXRob3JpdHkwHhcNMDYwOTE3MTk0NjM3WhcNMzYwOTE3MTk0NjM2WjB9
MQswCQYDVQQGEwJJTDEWMBQGA1UEChMNU3RhcnRDb20gTHRkLjErMCkGA1UECxMi
U2VjdXJlIERpZ2l0YWwgQ2VydGlmaWNhdGUgU2lnbmluZzEpMCcGA1UEAxMgU3Rh
cnRDb20gQ2VydGlmaWNhdGlvbiBBdXRob3JpdHkwggIiMA0GCSqGSIb3DQEBAQUA
A4ICDwAwggIKAoICAQDBiNsJvGxGfHiflXu1M5DycmLWwTYgIiRezul38kMKogZk
pMyONvg45iPwbm2xPN1yo4UcodM9tDMr0y+v/uqwQVlntsQGfQqedIXWeUyAN3rf
OQVSWff0G0ZDpNKFhdLDcfN1YjS6LIp/Ho/u7TTQEceWzVI9ujPW3U3eCztKS5/C
Ji/6tRYccjV3yjxd5srhJosaNnZcAdt0FCX+7bWgiA/deMotHweXMAEtcnn6RtYT
Kqi5pquDSR3l8u/d5AGOGAqPY1MWhWKpDhk6zLVmpsJrdAfkK+F2PrRt2PZE4XNi
HzvEvqBTViVsUQn3qqvKv3b9bZvzndu/PWa8DFaqr5hIlTpL36dYUNk4dalb6kMM
Av+Z6+hsTXBbKWWc3apdzK8BMewM69KN6Oqce+Zu9ydmDBpI125C4z/eIT574Q1w
+2OqqGwaVLRcJXrJosmLFqa7LH4XXgVNWG4SHQHuEhANxjJ/GP/89PrNbpHoNkm+
Gkhpi8KWTRoSsmkXwQqQ1vp5Iki/untp+HDH+no32NgN0nZPV/+Qt+OR0t3vwmC3
Zzrd/qqc8NSLf3Iizsafl7b4r4qgEKjZ+xjGtrVcUjyJthkqcwEKDwOzEmDyei+B
26Nu/yYwl/WL3YlXtq09s68rxbd2AvCl1iuahhQqcvbjM4xdCUsT37uMdBNSSwID
AQABo4ICEDCCAgwwDwYDVR0TAQH/BAUwAwEB/zAOBgNVHQ8BAf8EBAMCAQYwHQYD
VR0OBBYEFE4L7xqkQFulF2mHMMo0aEPQQa7yMB8GA1UdIwQYMBaAFE4L7xqkQFul
F2mHMMo0aEPQQa7yMIIBWgYDVR0gBIIBUTCCAU0wggFJBgsrBgEEAYG1NwEBATCC
ATgwLgYIKwYBBQUHAgEWImh0dHA6Ly93d3cuc3RhcnRzc2wuY29tL3BvbGljeS5w
ZGYwNAYIKwYBBQUHAgEWKGh0dHA6Ly93d3cuc3RhcnRzc2wuY29tL2ludGVybWVk
aWF0ZS5wZGYwgc8GCCsGAQUFBwICMIHCMCcWIFN0YXJ0IENvbW1lcmNpYWwgKFN0
YXJ0Q29tKSBMdGQuMAMCAQEagZZMaW1pdGVkIExpYWJpbGl0eSwgcmVhZCB0aGUg
c2VjdGlvbiAqTGVnYWwgTGltaXRhdGlvbnMqIG9mIHRoZSBTdGFydENvbSBDZXJ0
aWZpY2F0aW9uIEF1dGhvcml0eSBQb2xpY3kgYXZhaWxhYmxlIGF0IGh0dHA6Ly93
d3cuc3RhcnRzc2wuY29tL3BvbGljeS5wZGYwEQYJYIZIAYb4QgEBBAQDAgAHMDgG
CWCGSAGG+EIBDQQrFilTdGFydENvbSBGcmVlIFNTTCBDZXJ0aWZpY2F0aW9uIEF1
dGhvcml0eTANBgkqhkiG9w0BAQsFAAOCAgEAjo/n3JR5fPGFf59Jb2vKXfuM/gTF
wWLRfUKKvFO3lANmMD+x5wqnUCBVJX92ehQN6wQOQOY+2IirByeDqXWmN3PH/UvS
Ta0XQMhGvjt/UfzDtgUx3M2FIk5xt/JxXrAaxrqTi3iSSoX4eA+D/i+tLPfkpLst
0OcNOrg+zvZ49q5HJMqjNTbOx8aHmNrs++myziebiMMEofYLWWivydsQD032ZGNc
pRJvkrKTlMeIFw6Ttn5ii5B/q06f/ON1FE8qMt9bDeD1e5MNq6HPh+GlBEXoPBKl
CcWw0bdT82AUuoVpaiF8H3VhFyAXe2w7QSlc4axa0c2Mm+tgHRns9+Ww2vl5GKVF
P0lDV9LdJNUso/2RjSe15esUBppMeyG7Oq0wBhjA2MFrLH9ZXF2RsXAiV+uKa0hK
1Q8p7MZAwC+ITGgBF3f0JBlPvfrhsiAhS90a2Cl9qrjeVOwhVYBsHvUwyKMQ5bLm
KhQxw4UtjJixhlpPiVktucf3HMiKf8CdBUrmQk9io20ppB+Fq9vlgcitKj1MXVuE
JnHEhV5xJMqlG2zYYdMa4FTbzrqpMrUi9nNBCV24F10OD5mQ1kfabwo6YigUZ4LZ
8dCAWZvLMdibD4x3TrVoivJs9iQOLWxwxXPR3hTQcY+203sC9uO41Alua551hDnm
fyWl8kgAwKQB2j8=
-----END CERTIFICATE-----`

const smimeLeaf = `-----BEGIN CERTIFICATE-----
MIIFBjCCA+6gAwIBAgISESFvrjT8XcJTEe6rBlPptILlMA0GCSqGSIb3DQEBBQUA
MFQxCzAJBgNVBAYTAkJFMRkwFwYDVQQKExBHbG9iYWxTaWduIG52LXNhMSowKAYD
VQQDEyFHbG9iYWxTaWduIFBlcnNvbmFsU2lnbiAyIENBIC0gRzIwHhcNMTIwMTIz
MTYzNjU5WhcNMTUwMTIzMTYzNjU5WjCBlDELMAkGA1UEBhMCVVMxFjAUBgNVBAgT
DU5ldyBIYW1zcGhpcmUxEzARBgNVBAcTClBvcnRzbW91dGgxGTAXBgNVBAoTEEds
b2JhbFNpZ24sIEluYy4xEzARBgNVBAMTClJ5YW4gSHVyc3QxKDAmBgkqhkiG9w0B
CQEWGXJ5YW4uaHVyc3RAZ2xvYmFsc2lnbi5jb20wggEiMA0GCSqGSIb3DQEBAQUA
A4IBDwAwggEKAoIBAQC4ASSTvavmsFQAob60ukSSwOAL9nT/s99ltNUCAf5fPH5j
NceMKxaQse2miOmRRIXaykcq1p/TbI70Ztce38r2mbOwqDHHPVi13GxJEyUXWgaR
BteDMu5OGyWNG1kchVsGWpbstT0Z4v0md5m1BYFnxB20ebJyOR2lXDxsFK28nnKV
+5eMj76U8BpPQ4SCH7yTMG6y0XXsB3cCrBKr2o3TOYgEKv+oNnbaoMt3UxMt9nSf
9jyIshjqfnT5Aew3CUNMatO55g5FXXdIukAweg1YSb1ls05qW3sW00T3d7dQs9/7
NuxCg/A2elmVJSoy8+MLR8JSFEf/aMgjO/TyLg/jAgMBAAGjggGPMIIBizAOBgNV
HQ8BAf8EBAMCBaAwTQYDVR0gBEYwRDBCBgorBgEEAaAyASgKMDQwMgYIKwYBBQUH
AgEWJmh0dHBzOi8vd3d3Lmdsb2JhbHNpZ24uY29tL3JlcG9zaXRvcnkvMCQGA1Ud
EQQdMBuBGXJ5YW4uaHVyc3RAZ2xvYmFsc2lnbi5jb20wCQYDVR0TBAIwADAdBgNV
HSUEFjAUBggrBgEFBQcDAgYIKwYBBQUHAwQwQwYDVR0fBDwwOjA4oDagNIYyaHR0
cDovL2NybC5nbG9iYWxzaWduLmNvbS9ncy9nc3BlcnNvbmFsc2lnbjJnMi5jcmww
VQYIKwYBBQUHAQEESTBHMEUGCCsGAQUFBzAChjlodHRwOi8vc2VjdXJlLmdsb2Jh
bHNpZ24uY29tL2NhY2VydC9nc3BlcnNvbmFsc2lnbjJnMi5jcnQwHQYDVR0OBBYE
FFWiECe0/L72eVYqcWYnLV6SSjzhMB8GA1UdIwQYMBaAFD8V0m18L+cxnkMKBqiU
bCw7xe5lMA0GCSqGSIb3DQEBBQUAA4IBAQAhQi6hLPeudmf3IBF4IDzCvRI0FaYd
BKfprSk/H0PDea4vpsLbWpA0t0SaijiJYtxKjlM4bPd+2chb7ejatDdyrZIzmDVy
q4c30/xMninGKokpYA11/Ve+i2dvjulu65qasrtQRGybAuuZ67lrp/K3OMFgjV5N
C3AHYLzvNU4Dwc4QQ1BaMOg6KzYSrKbABRZajfrpC9uiePsv7mDIXLx/toBPxWNl
a5vJm5DrZdn7uHdvBCE6kMykbOLN5pmEK0UIlwKh6Qi5XD0pzlVkEZliFkBMJgub
d/eF7xeg7TKPWC5xyOFp9SdMolJM7LTC3wnSO3frBAev+q/nGs9Xxyvs
-----END CERTIFICATE-----`

const smimeIntermediate = `-----BEGIN CERTIFICATE-----
MIIEFjCCAv6gAwIBAgILBAAAAAABL07hL1IwDQYJKoZIhvcNAQEFBQAwVzELMAkG
A1UEBhMCQkUxGTAXBgNVBAoTEEdsb2JhbFNpZ24gbnYtc2ExEDAOBgNVBAsTB1Jv
b3QgQ0ExGzAZBgNVBAMTEkdsb2JhbFNpZ24gUm9vdCBDQTAeFw0xMTA0MTMxMDAw
MDBaFw0xOTA0MTMxMDAwMDBaMFQxCzAJBgNVBAYTAkJFMRkwFwYDVQQKExBHbG9i
YWxTaWduIG52LXNhMSowKAYDVQQDEyFHbG9iYWxTaWduIFBlcnNvbmFsU2lnbiAy
IENBIC0gRzIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDBa0H5Nez4
En3dIlFpX7e5E0YndxQ74xOBbz7kdBd+DLX0LOQMjVPU3DAgKL9ujhH+ZhHkURbH
3X/94TQSUL/z2JjsaQvS0NqyZXHhM5eeuquzOJRzEQ8+odETzHg2G0Erv7yjSeww
gkwDWDJnYUDlOjYTDUEG6+i+8Mn425reo4I0E277wD542kmVWeW7+oHv5dZo9e1Q
yWwiKTEP6BEQVVSBgThXMG4traSSDRUt3T1eQTZx5EObpiBEBO4OTqiBTJfg4vEI
YgkXzKLpnfszTB6YMDpR9/QS6p3ANB3kfAb+t6udSO3WCst0DGrwHDLBFGDR4UeY
T5KGGnI7cWL7AgMBAAGjgeUwgeIwDgYDVR0PAQH/BAQDAgEGMBIGA1UdEwEB/wQI
MAYBAf8CAQAwHQYDVR0OBBYEFD8V0m18L+cxnkMKBqiUbCw7xe5lMEcGA1UdIARA
MD4wPAYEVR0gADA0MDIGCCsGAQUFBwIBFiZodHRwczovL3d3dy5nbG9iYWxzaWdu
LmNvbS9yZXBvc2l0b3J5LzAzBgNVHR8ELDAqMCigJqAkhiJodHRwOi8vY3JsLmds
b2JhbHNpZ24ubmV0L3Jvb3QuY3JsMB8GA1UdIwQYMBaAFGB7ZhpFDZfKiVAvfQTN
NKj//P1LMA0GCSqGSIb3DQEBBQUAA4IBAQBDc3nMpMxJMQMcYUCB3+C73UpvwDE8
eCOr7t2F/uaQKKcyqqstqLZc6vPwI/rcE9oDHugY5QEjQzIBIEaTnN6P0vege2IX
eCOr7t2F/uaQKKcyqqstqLZc6vPwI/rcE9oDHugY5QEjQzIBIEaTnN6P0vege2IX
YEvTWbWwGdPytDFPYIl3/6OqNSXSnZ7DxPcdLJq2uyiga8PB/TTIIHYkdM2+1DE0
7y3rH/7TjwDVD7SLu5/SdOfKskuMPTjOEvz3K161mymW06klVhubCIWOro/Gx1Q2
2FQOZ7/2k4uYoOdBTSlb8kTAuzZNgIE0rB2BIYCTz/P6zZIKW0ogbRSH
-----END CERTIFICATE-----`

var megaLeaf = `-----BEGIN CERTIFICATE-----
MIIFOjCCBCKgAwIBAgIQWYE8Dup170kZ+k11Lg51OjANBgkqhkiG9w0BAQUFADBy
MQswCQYDVQQGEwJHQjEbMBkGA1UECBMSR3JlYXRlciBNYW5jaGVzdGVyMRAwDgYD
VQQHEwdTYWxmb3JkMRowGAYDVQQKExFDT01PRE8gQ0EgTGltaXRlZDEYMBYGA1UE
AxMPRXNzZW50aWFsU1NMIENBMB4XDTEyMTIxNDAwMDAwMFoXDTE0MTIxNDIzNTk1
OVowfzEhMB8GA1UECxMYRG9tYWluIENvbnRyb2wgVmFsaWRhdGVkMS4wLAYDVQQL
EyVIb3N0ZWQgYnkgSW5zdHJhIENvcnBvcmF0aW9uIFB0eS4gTFREMRUwEwYDVQQL
EwxFc3NlbnRpYWxTU0wxEzARBgNVBAMTCm1lZ2EuY28ubnowggEiMA0GCSqGSIb3
DQEBAQUAA4IBDwAwggEKAoIBAQDcxMCClae8BQIaJHBUIVttlLvhbK4XhXPk3RQ3
G5XA6tLZMBQ33l3F9knYJ0YErXtr8IdfYoulRQFmKFMJl9GtWyg4cGQi2Rcr5VN5
S5dA1vu4oyJBxE9fPELcK6Yz1vqaf+n6za+mYTiQYKggVdS8/s8hmNuXP9Zk1pIn
+q0pGsf8NAcSHMJgLqPQrTDw+zae4V03DvcYfNKjuno88d2226ld7MAmQZ7uRNsI
/CnkdelVs+akZsXf0szefSqMJlf08SY32t2jj4Ra7RApVYxOftD9nij/aLfuqOU6
ow6IgIcIG2ZvXLZwK87c5fxL7UAsTTV+M1sVv8jA33V2oKLhAgMBAAGjggG9MIIB
uTAfBgNVHSMEGDAWgBTay+qtWwhdzP/8JlTOSeVVxjj0+DAdBgNVHQ4EFgQUmP9l
6zhyrZ06Qj4zogt+6LKFk4AwDgYDVR0PAQH/BAQDAgWgMAwGA1UdEwEB/wQCMAAw
NAYDVR0lBC0wKwYIKwYBBQUHAwEGCCsGAQUFBwMCBgorBgEEAYI3CgMDBglghkgB
hvhCBAEwTwYDVR0gBEgwRjA6BgsrBgEEAbIxAQICBzArMCkGCCsGAQUFBwIBFh1o
dHRwczovL3NlY3VyZS5jb21vZG8uY29tL0NQUzAIBgZngQwBAgEwOwYDVR0fBDQw
MjAwoC6gLIYqaHR0cDovL2NybC5jb21vZG9jYS5jb20vRXNzZW50aWFsU1NMQ0Eu
Y3JsMG4GCCsGAQUFBwEBBGIwYDA4BggrBgEFBQcwAoYsaHR0cDovL2NydC5jb21v
ZG9jYS5jb20vRXNzZW50aWFsU1NMQ0FfMi5jcnQwJAYIKwYBBQUHMAGGGGh0dHA6
Ly9vY3NwLmNvbW9kb2NhLmNvbTAlBgNVHREEHjAcggptZWdhLmNvLm56gg53d3cu
bWVnYS5jby5uejANBgkqhkiG9w0BAQUFAAOCAQEAcYhrsPSvDuwihMOh0ZmRpbOE
Gw6LqKgLNTmaYUPQhzi2cyIjhUhNvugXQQlP5f0lp5j8cixmArafg1dTn4kQGgD3
ivtuhBTgKO1VYB/VRoAt6Lmswg3YqyiS7JiLDZxjoV7KoS5xdiaINfHDUaBBY4ZH
j2BUlPniNBjCqXe/HndUTVUewlxbVps9FyCmH+C4o9DWzdGBzDpCkcmo5nM+cp7q
ZhTIFTvZfo3zGuBoyu8BzuopCJcFRm3cRiXkpI7iOMUIixO1szkJS6WpL1sKdT73
UXp08U0LBqoqG130FbzEJBBV3ixbvY6BWMHoCWuaoF12KJnC5kHt2RoWAAgMXA==
-----END CERTIFICATE-----`

var comodoIntermediate1 = `-----BEGIN CERTIFICATE-----
MIIFAzCCA+ugAwIBAgIQGLLLuqME8aAPwfLzJkYqSjANBgkqhkiG9w0BAQUFADCB
gTELMAkGA1UEBhMCR0IxGzAZBgNVBAgTEkdyZWF0ZXIgTWFuY2hlc3RlcjEQMA4G
A1UEBxMHU2FsZm9yZDEaMBgGA1UEChMRQ09NT0RPIENBIExpbWl0ZWQxJzAlBgNV
BAMTHkNPTU9ETyBDZXJ0aWZpY2F0aW9uIEF1dGhvcml0eTAeFw0wNjEyMDEwMDAw
MDBaFw0xOTEyMzEyMzU5NTlaMHIxCzAJBgNVBAYTAkdCMRswGQYDVQQIExJHcmVh
dGVyIE1hbmNoZXN0ZXIxEDAOBgNVBAcTB1NhbGZvcmQxGjAYBgNVBAoTEUNPTU9E
TyBDQSBMaW1pdGVkMRgwFgYDVQQDEw9Fc3NlbnRpYWxTU0wgQ0EwggEiMA0GCSqG
SIb3DQEBAQUAA4IBDwAwggEKAoIBAQCt8AiwcsargxIxF3CJhakgEtSYau2A1NHf
5I5ZLdOWIY120j8YC0YZYwvHIPPlC92AGvFaoL0dds23Izp0XmEbdaqb1IX04XiR
0y3hr/yYLgbSeT1awB8hLRyuIVPGOqchfr7tZ291HRqfalsGs2rjsQuqag7nbWzD
ypWMN84hHzWQfdvaGlyoiBSyD8gSIF/F03/o4Tjg27z5H6Gq1huQByH6RSRQXScq
oChBRVt9vKCiL6qbfltTxfEFFld+Edc7tNkBdtzffRDPUanlOPJ7FAB1WfnwWdsX
Pvev5gItpHnBXaIcw5rIp6gLSApqLn8tl2X2xQScRMiZln5+pN0vAgMBAAGjggGD
MIIBfzAfBgNVHSMEGDAWgBQLWOWLxkwVN6RAqTCpIb5HNlpW/zAdBgNVHQ4EFgQU
2svqrVsIXcz//CZUzknlVcY49PgwDgYDVR0PAQH/BAQDAgEGMBIGA1UdEwEB/wQI
MAYBAf8CAQAwIAYDVR0lBBkwFwYKKwYBBAGCNwoDAwYJYIZIAYb4QgQBMD4GA1Ud
IAQ3MDUwMwYEVR0gADArMCkGCCsGAQUFBwIBFh1odHRwczovL3NlY3VyZS5jb21v
ZG8uY29tL0NQUzBJBgNVHR8EQjBAMD6gPKA6hjhodHRwOi8vY3JsLmNvbW9kb2Nh
LmNvbS9DT01PRE9DZXJ0aWZpY2F0aW9uQXV0aG9yaXR5LmNybDBsBggrBgEFBQcB
AQRgMF4wNgYIKwYBBQUHMAKGKmh0dHA6Ly9jcnQuY29tb2RvY2EuY29tL0NvbW9k
b1VUTlNHQ0NBLmNydDAkBggrBgEFBQcwAYYYaHR0cDovL29jc3AuY29tb2RvY2Eu
Y29tMA0GCSqGSIb3DQEBBQUAA4IBAQAtlzR6QDLqcJcvgTtLeRJ3rvuq1xqo2l/z
odueTZbLN3qo6u6bldudu+Ennv1F7Q5Slqz0J790qpL0pcRDAB8OtXj5isWMcL2a
ejGjKdBZa0wztSz4iw+SY1dWrCRnilsvKcKxudokxeRiDn55w/65g+onO7wdQ7Vu
F6r7yJiIatnyfKH2cboZT7g440LX8NqxwCPf3dfxp+0Jj1agq8MLy6SSgIGSH6lv
+Wwz3D5XxqfyH8wqfOQsTEZf6/Nh9yvENZ+NWPU6g0QO2JOsTGvMd/QDzczc4BxL
XSXaPV7Od4rhPsbXlM1wSTz/Dr0ISKvlUhQVnQ6cGodWaK2cCQBk
-----END CERTIFICATE-----`

var comodoRoot = `-----BEGIN CERTIFICATE-----
MIIEHTCCAwWgAwIBAgIQToEtioJl4AsC7j41AkblPTANBgkqhkiG9w0BAQUFADCB
gTELMAkGA1UEBhMCR0IxGzAZBgNVBAgTEkdyZWF0ZXIgTWFuY2hlc3RlcjEQMA4G
A1UEBxMHU2FsZm9yZDEaMBgGA1UEChMRQ09NT0RPIENBIExpbWl0ZWQxJzAlBgNV
BAMTHkNPTU9ETyBDZXJ0aWZpY2F0aW9uIEF1dGhvcml0eTAeFw0wNjEyMDEwMDAw
MDBaFw0yOTEyMzEyMzU5NTlaMIGBMQswCQYDVQQGEwJHQjEbMBkGA1UECBMSR3Jl
YXRlciBNYW5jaGVzdGVyMRAwDgYDVQQHEwdTYWxmb3JkMRowGAYDVQQKExFDT01P
RE8gQ0EgTGltaXRlZDEnMCUGA1UEAxMeQ09NT0RPIENlcnRpZmljYXRpb24gQXV0
aG9yaXR5MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA0ECLi3LjkRv3
UcEbVASY06m/weaKXTuH+7uIzg3jLz8GlvCiKVCZrts7oVewdFFxze1CkU1B/qnI
2GqGd0S7WWaXUF601CxwRM/aN5VCaTwwxHGzUvAhTaHYujl8HJ6jJJ3ygxaYqhZ8
Q5sVW7euNJH+1GImGEaaP+vB+fGQV+useg2L23IwambV4EajcNxo2f8ESIl33rXp
+2dtQem8Ob0y2WIC8bGoPW43nOIv4tOiJovGuFVDiOEjPqXSJDlqR6sA1KGzqSX+
DT+nHbrTUcELpNqsOO9VUCQFZUaTNE8tja3G1CEZ0o7KBWFxB3NH5YoZEr0ETc5O
nKVIrLsm9wIDAQABo4GOMIGLMB0GA1UdDgQWBBQLWOWLxkwVN6RAqTCpIb5HNlpW
/zAOBgNVHQ8BAf8EBAMCAQYwDwYDVR0TAQH/BAUwAwEB/zBJBgNVHR8EQjBAMD6g
PKA6hjhodHRwOi8vY3JsLmNvbW9kb2NhLmNvbS9DT01PRE9DZXJ0aWZpY2F0aW9u
QXV0aG9yaXR5LmNybDANBgkqhkiG9w0BAQUFAAOCAQEAPpiem/Yb6dc5t3iuHXIY
SdOH5EOC6z/JqvWote9VfCFSZfnVDeFs9D6Mk3ORLgLETgdxb8CPOGEIqB6BCsAv
IC9Bi5HcSEW88cbeunZrM8gALTFGTO3nnc+IlP8zwFboJIYmuNg4ON8qa90SzMc/
RxdMosIGlgnW2/4/PEZB31jiVg88O8EckzXZOFKs7sjsLjBOlDW0JB9LeGna8gI4
zJVSk/BwJVmcIGfE7vmLV2H0knZ9P4SNVbfo5azV8fUZVqZa+5Acr5Pr5RzUZ5dd
BA6+C4OmF4O5MBKgxTMVBbkN+8cFduPYSo38NBejxiEovjBFMR7HeL5YYTisO+IB
ZQ==
-----END CERTIFICATE-----`

var nameConstraintsLeaf = `-----BEGIN CERTIFICATE-----
MIIHMTCCBRmgAwIBAgIIIZaV/3ezOJkwDQYJKoZIhvcNAQEFBQAwgcsxCzAJBgNV
BAYTAlVTMREwDwYDVQQIEwhWaXJnaW5pYTETMBEGA1UEBxMKQmxhY2tzYnVyZzEj
MCEGA1UECxMaR2xvYmFsIFF1YWxpZmllZCBTZXJ2ZXIgQ0ExPDA6BgNVBAoTM1Zp
cmdpbmlhIFBvbHl0ZWNobmljIEluc3RpdHV0ZSBhbmQgU3RhdGUgVW5pdmVyc2l0
eTExMC8GA1UEAxMoVmlyZ2luaWEgVGVjaCBHbG9iYWwgUXVhbGlmaWVkIFNlcnZl
ciBDQTAeFw0xMzA5MTkxNDM2NTVaFw0xNTA5MTkxNDM2NTVaMIHNMQswCQYDVQQG
EwJVUzERMA8GA1UECAwIVmlyZ2luaWExEzARBgNVBAcMCkJsYWNrc2J1cmcxPDA6
BgNVBAoMM1ZpcmdpbmlhIFBvbHl0ZWNobmljIEluc3RpdHV0ZSBhbmQgU3RhdGUg
VW5pdmVyc2l0eTE7MDkGA1UECwwyVGVjaG5vbG9neS1lbmhhbmNlZCBMZWFybmlu
ZyBhbmQgT25saW5lIFN0cmF0ZWdpZXMxGzAZBgNVBAMMEnNlY3VyZS5pZGRsLnZ0
LmVkdTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAKkOyPpsOK/6IuPG
WnIBlVwlHzeYf+cUlggqkLq0b0+vZbiTXgio9/VCuNQ8opSoss7J7o3ygV9to+9Y
YwJKVC5WDT/y5JWpQey0CWILymViJnpNSwnxBc8A+Q8w5NUGDd/UhtPx/U8/hqbd
WPDYj2hbOqyq8UlRhfS5pwtnv6BbCTaY11I6FhCLK7zttISyTuWCf9p9o/ggiipP
ii/5oh4dkl+r5SfuSp5GPNHlYO8lWqys5NAPoDD4fc/kuflcK7Exx7XJ+Oqu0W0/
psjEY/tES1ZgDWU/ParcxxFpFmKHbD5DXsfPOObzkVWXIY6tGMutSlE1Froy/Nn0
OZsAOrcCAwEAAaOCAhMwggIPMIG4BggrBgEFBQcBAQSBqzCBqDBYBggrBgEFBQcw
AoZMaHR0cDovL3d3dy5wa2kudnQuZWR1L2dsb2JhbHF1YWxpZmllZHNlcnZlci9j
YWNlcnQvZ2xvYmFscXVhbGlmaWVkc2VydmVyLmNydDBMBggrBgEFBQcwAYZAaHR0
cDovL3Z0Y2EtcC5lcHJvdi5zZXRpLnZ0LmVkdTo4MDgwL2VqYmNhL3B1YmxpY3dl
Yi9zdGF0dXMvb2NzcDAdBgNVHQ4EFgQUp7xbO6iHkvtZbPE4jmndmnAbSEcwDAYD
VR0TAQH/BAIwADAfBgNVHSMEGDAWgBS8YmAn1eM1SBfpS6tFatDIqHdxjDBqBgNV
HSAEYzBhMA4GDCsGAQQBtGgFAgICATAOBgwrBgEEAbRoBQICAQEwPwYMKwYBBAG0
aAUCAgMBMC8wLQYIKwYBBQUHAgEWIWh0dHA6Ly93d3cucGtpLnZ0LmVkdS9nbG9i
YWwvY3BzLzBKBgNVHR8EQzBBMD+gPaA7hjlodHRwOi8vd3d3LnBraS52dC5lZHUv
Z2xvYmFscXVhbGlmaWVkc2VydmVyL2NybC9jYWNybC5jcmwwDgYDVR0PAQH/BAQD
AgTwMB0GA1UdJQQWMBQGCCsGAQUFBwMBBggrBgEFBQcDAjAdBgNVHREEFjAUghJz
ZWN1cmUuaWRkbC52dC5lZHUwDQYJKoZIhvcNAQEFBQADggIBAEgoYo4aUtatY3gI
OyyKp7QlIOaLbTJZywESHqy+L5EGDdJW2DJV+mcE0LDGvqa2/1Lo+AR1ntsZwfOi
Y718JwgVVaX/RCd5+QKP25c5/x72xI8hb/L1bgS0ED9b0YAhd7Qm1K1ot82+6mqX
DW6WiGeDr8Z07MQ3143qQe2rBlq+QI69DYzm2GOqAIAnUIWv7tCyLUm31b4DwmrJ
TeudVreTKUbBNB1TWRFHEPkWhjjXKZnNGRO11wHXcyBu6YekIvVZ+vmx8ePee4jJ
3GFOi7lMuWOeq57jTVL7KOKaKLVXBb6gqo5aq+Wwt8RUD5MakrCAEeQZj7DKaFmZ
oQCO0Pxrsl3InCGvxnGzT+bFVO9nJ/BAMj7hknFdm9Jr6Bg5q33Z+gnf909AD9QF
ESqUSykaHu2LVdJx2MaCH1CyKnRgMw5tEwE15EXpUjCm24m8FMOYC+rNtf18pgrz
5D8Jhh+oxK9PjcBYqXNtnioIxiMCYcV0q5d4w4BYFEh71tk7/bYB0R55CsBUVPmp
timWNOdRd57Tfpk3USaVsumWZAf9MP3wPiC7gb4d5tYEEAG5BuDT8ruFw838wU8G
1VvAVutSiYBg7k3NYO7AUqZ+Ax4klQX3aM9lgonmJ78Qt94UPtbptrfZ4/lSqEf8
GBUwDrQNTb+gsXsDkjd5lcYxNx6l
-----END CERTIFICATE-----`

var nameConstraintsIntermediate1 = `-----BEGIN CERTIFICATE-----
MIINLjCCDBagAwIBAgIRIqpyf/YoGgvHc8HiDAxAI8owDQYJKoZIhvcNAQEFBQAw
XDELMAkGA1UEBhMCQkUxFTATBgNVBAsTDFRydXN0ZWQgUm9vdDEZMBcGA1UEChMQ
R2xvYmFsU2lnbiBudi1zYTEbMBkGA1UEAxMSVHJ1c3RlZCBSb290IENBIEcyMB4X
DTEyMTIxMzAwMDAwMFoXDTE3MTIxMzAwMDAwMFowgcsxCzAJBgNVBAYTAlVTMREw
DwYDVQQIEwhWaXJnaW5pYTETMBEGA1UEBxMKQmxhY2tzYnVyZzEjMCEGA1UECxMa
R2xvYmFsIFF1YWxpZmllZCBTZXJ2ZXIgQ0ExPDA6BgNVBAoTM1ZpcmdpbmlhIFBv
bHl0ZWNobmljIEluc3RpdHV0ZSBhbmQgU3RhdGUgVW5pdmVyc2l0eTExMC8GA1UE
AxMoVmlyZ2luaWEgVGVjaCBHbG9iYWwgUXVhbGlmaWVkIFNlcnZlciBDQTCCAiIw
DQYJKoZIhvcNAQEBBQADggIPADCCAgoCggIBALgIZhEaptBWADBqdJ45ueFGzMXa
GHnzNxoxR1fQIaaRQNdCg4cw3A4dWKMeEgYLtsp65ai3Xfw62Qaus0+KJ3RhgV+r
ihqK81NUzkls78fJlADVDI4fCTlothsrE1CTOMiy97jKHai5mVTiWxmcxpmjv7fm
5Nhc+uHgh2hIz6npryq495mD51ZrUTIaqAQN6Pw/VHfAmR524vgriTOjtp1t4lA9
pXGWjF/vkhAKFFheOQSQ00rngo2wHgCqMla64UTN0oz70AsCYNZ3jDLx0kOP0YmM
R3Ih91VA63kLqPXA0R6yxmmhhxLZ5bcyAy1SLjr1N302MIxLM/pSy6aquEnbELhz
qyp9yGgRyGJay96QH7c4RJY6gtcoPDbldDcHI9nXngdAL4DrZkJ9OkDkJLyqG66W
ZTF5q4EIs6yMdrywz0x7QP+OXPJrjYpbeFs6tGZCFnWPFfmHCRJF8/unofYrheq+
9J7Jx3U55S/k57NXbAM1RAJOuMTlfn9Etf9Dpoac9poI4Liav6rBoUQk3N3JWqnV
HNx/NdCyJ1/6UbKMJUZsStAVglsi6lVPo289HHOE4f7iwl3SyekizVOp01wUin3y
cnbZB/rXmZbwapSxTTSBf0EIOr9i4EGfnnhCAVA9U5uLrI5OEB69IY8PNX0071s3
Z2a2fio5c8m3JkdrAgMBAAGjggh5MIIIdTAOBgNVHQ8BAf8EBAMCAQYwTAYDVR0g
BEUwQzBBBgkrBgEEAaAyATwwNDAyBggrBgEFBQcCARYmaHR0cHM6Ly93d3cuZ2xv
YmFsc2lnbi5jb20vcmVwb3NpdG9yeS8wEgYDVR0TAQH/BAgwBgEB/wIBADCCBtAG
A1UdHgSCBscwggbDoIIGvzASghAzZGJsYWNrc2J1cmcub3JnMBiCFmFjY2VsZXJh
dGV2aXJnaW5pYS5jb20wGIIWYWNjZWxlcmF0ZXZpcmdpbmlhLm9yZzALgglhY3Zj
cC5vcmcwCYIHYmV2Lm5ldDAJggdiZXYub3JnMAuCCWNsaWdzLm9yZzAMggpjbWl3
ZWIub3JnMBeCFWVhc3Rlcm5icm9va3Ryb3V0Lm5ldDAXghVlYXN0ZXJuYnJvb2t0
cm91dC5vcmcwEYIPZWNvcnJpZG9ycy5pbmZvMBOCEWVkZ2FycmVzZWFyY2gub3Jn
MBKCEGdldC1lZHVjYXRlZC5jb20wE4IRZ2V0LWVkdWNhdGVkLmluZm8wEYIPZ2V0
ZWR1Y2F0ZWQubmV0MBKCEGdldC1lZHVjYXRlZC5uZXQwEYIPZ2V0ZWR1Y2F0ZWQu
b3JnMBKCEGdldC1lZHVjYXRlZC5vcmcwD4INaG9raWVjbHViLmNvbTAQgg5ob2tp
ZXBob3RvLmNvbTAPgg1ob2tpZXNob3AuY29tMBGCD2hva2llc3BvcnRzLmNvbTAS
ghBob2tpZXRpY2tldHMuY29tMBKCEGhvdGVscm9hbm9rZS5jb20wE4IRaHVtYW53
aWxkbGlmZS5vcmcwF4IVaW5uYXR2aXJnaW5pYXRlY2guY29tMA+CDWlzY2hwMjAx
MS5vcmcwD4INbGFuZHJlaGFiLm9yZzAggh5uYXRpb25hbHRpcmVyZXNlYXJjaGNl
bnRlci5jb20wFYITbmV0d29ya3ZpcmdpbmlhLm5ldDAMggpwZHJjdnQuY29tMBiC
FnBldGVkeWVyaXZlcmNvdXJzZS5jb20wDYILcmFkaW9pcS5vcmcwFYITcml2ZXJj
b3Vyc2Vnb2xmLmNvbTALgglzZGltaS5vcmcwEIIOc292YW1vdGlvbi5jb20wHoIc
c3VzdGFpbmFibGUtYmlvbWF0ZXJpYWxzLmNvbTAeghxzdXN0YWluYWJsZS1iaW9t
YXRlcmlhbHMub3JnMBWCE3RoaXNpc3RoZWZ1dHVyZS5jb20wGIIWdGhpcy1pcy10
aGUtZnV0dXJlLmNvbTAVghN0aGlzaXN0aGVmdXR1cmUubmV0MBiCFnRoaXMtaXMt
dGhlLWZ1dHVyZS5uZXQwCoIIdmFkcy5vcmcwDIIKdmFsZWFmLm9yZzANggt2YXRl
Y2guaW5mbzANggt2YXRlY2gubW9iaTAcghp2YXRlY2hsaWZlbG9uZ2xlYXJuaW5n
LmNvbTAcghp2YXRlY2hsaWZlbG9uZ2xlYXJuaW5nLm5ldDAcghp2YXRlY2hsaWZl
bG9uZ2xlYXJuaW5nLm9yZzAKggh2Y29tLmVkdTASghB2aXJnaW5pYXZpZXcubmV0
MDSCMnZpcmdpbmlhcG9seXRlY2huaWNpbnN0aXR1dGVhbmRzdGF0ZXVuaXZlcnNp
dHkuY29tMDWCM3ZpcmdpbmlhcG9seXRlY2huaWNpbnN0aXR1dGVhbmRzdGF0ZXVu
aXZlcnNpdHkuaW5mbzA0gjJ2aXJnaW5pYXBvbHl0ZWNobmljaW5zdGl0dXRlYW5k
c3RhdGV1bml2ZXJzaXR5Lm5ldDA0gjJ2aXJnaW5pYXBvbHl0ZWNobmljaW5zdGl0
dXRlYW5kc3RhdGV1bml2ZXJzaXR5Lm9yZzAZghd2aXJnaW5pYXB1YmxpY3JhZGlv
Lm9yZzASghB2aXJnaW5pYXRlY2guZWR1MBOCEXZpcmdpbmlhdGVjaC5tb2JpMByC
GnZpcmdpbmlhdGVjaGZvdW5kYXRpb24ub3JnMAiCBnZ0LmVkdTALggl2dGFyYy5v
cmcwDIIKdnQtYXJjLm9yZzALggl2dGNyYy5jb20wCoIIdnRpcC5vcmcwDIIKdnRs
ZWFuLm9yZzAWghR2dGtub3dsZWRnZXdvcmtzLmNvbTAYghZ2dGxpZmVsb25nbGVh
cm5pbmcuY29tMBiCFnZ0bGlmZWxvbmdsZWFybmluZy5uZXQwGIIWdnRsaWZlbG9u
Z2xlYXJuaW5nLm9yZzATghF2dHNwb3J0c21lZGlhLmNvbTALggl2dHdlaS5jb20w
D4INd2l3YXR3ZXJjLmNvbTAKggh3dnRmLm9yZzAIgQZ2dC5lZHUwd6R1MHMxCzAJ
BgNVBAYTAlVTMREwDwYDVQQIEwhWaXJnaW5pYTETMBEGA1UEBxMKQmxhY2tzYnVy
ZzE8MDoGA1UEChMzVmlyZ2luaWEgUG9seXRlY2huaWMgSW5zdGl0dXRlIGFuZCBT
dGF0ZSBVbml2ZXJzaXR5MCcGA1UdJQQgMB4GCCsGAQUFBwMCBggrBgEFBQcDAQYI
KwYBBQUHAwkwPQYDVR0fBDYwNDAyoDCgLoYsaHR0cDovL2NybC5nbG9iYWxzaWdu
LmNvbS9ncy90cnVzdHJvb3RnMi5jcmwwgYQGCCsGAQUFBwEBBHgwdjAzBggrBgEF
BQcwAYYnaHR0cDovL29jc3AyLmdsb2JhbHNpZ24uY29tL3RydXN0cm9vdGcyMD8G
CCsGAQUFBzAChjNodHRwOi8vc2VjdXJlLmdsb2JhbHNpZ24uY29tL2NhY2VydC90
cnVzdHJvb3RnMi5jcnQwHQYDVR0OBBYEFLxiYCfV4zVIF+lLq0Vq0Miod3GMMB8G
A1UdIwQYMBaAFBT25YsxtkWASkxt/MKHico2w5BiMA0GCSqGSIb3DQEBBQUAA4IB
AQAyJm/lOB2Er4tHXhc/+fSufSzgjohJgYfMkvG4LknkvnZ1BjliefR8tTXX49d2
SCDFWfGjqyJZwavavkl/4p3oXPG/nAMDMvxh4YAT+CfEK9HH+6ICV087kD4BLegi
+aFJMj8MMdReWCzn5sLnSR1rdse2mo2arX3Uod14SW+PGrbUmTuWNyvRbz3fVmxp
UdbGmj3laknO9YPsBGgHfv73pVVsTJkW4ZfY/7KdD/yaVv6ophpOB3coXfjl2+kd
Z4ypn2zK+cx9IL/LSewqd/7W9cD55PCUy4X9OTbEmAccwiz3LB66mQoUGfdHdkoB
jUY+v9vLQXmaVwI0AYL7g9LN
-----END CERTIFICATE-----`

var nameConstraintsIntermediate2 = `-----BEGIN CERTIFICATE-----
MIIEXTCCA0WgAwIBAgILBAAAAAABNuk6OrMwDQYJKoZIhvcNAQEFBQAwVzELMAkG
A1UEBhMCQkUxGTAXBgNVBAoTEEdsb2JhbFNpZ24gbnYtc2ExEDAOBgNVBAsTB1Jv
b3QgQ0ExGzAZBgNVBAMTEkdsb2JhbFNpZ24gUm9vdCBDQTAeFw0xMjA0MjUxMTAw
MDBaFw0yNzA0MjUxMTAwMDBaMFwxCzAJBgNVBAYTAkJFMRUwEwYDVQQLEwxUcnVz
dGVkIFJvb3QxGTAXBgNVBAoTEEdsb2JhbFNpZ24gbnYtc2ExGzAZBgNVBAMTElRy
dXN0ZWQgUm9vdCBDQSBHMjCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEB
AKyuvqrtcMr7g7EuNbu4sKwxM127UsCmx1RxbxxgcArGS7rjiefpBH/w4LYrymjf
vcw1ueyMNoqLo9nJMz/ORXupb35NNfE667prQYHa+tTjl1IiKpB7QUwt3wXPuTMF
Ja1tXtjKzkqJyuJlNuPKT76HcjgNqgV1s9qG44MD5I2JvI12du8zI1bgdQ+l/KsX
kTfbGjUvhOLOlVNWVQDpL+YMIrGqgBYxy5TUNgrAcRtwpNdS2KkF5otSmMweVb5k
hoUVv3u8UxQH/WWbNhHq1RrIlg/0rBUfi/ziShYFSB7U+aLx5DxPphTFBiDquQGp
tB+FC4JvnukDStFihZCZ1R8CAwEAAaOCASMwggEfMA4GA1UdDwEB/wQEAwIBBjAP
BgNVHRMBAf8EBTADAQH/MEcGA1UdIARAMD4wPAYEVR0gADA0MDIGCCsGAQUFBwIB
FiZodHRwczovL3d3dy5nbG9iYWxzaWduLmNvbS9yZXBvc2l0b3J5LzAdBgNVHQ4E
FgQUFPblizG2RYBKTG38woeJyjbDkGIwMwYDVR0fBCwwKjAooCagJIYiaHR0cDov
L2NybC5nbG9iYWxzaWduLm5ldC9yb290LmNybDA+BggrBgEFBQcBAQQyMDAwLgYI
KwYBBQUHMAGGImh0dHA6Ly9vY3NwMi5nbG9iYWxzaWduLmNvbS9yb290cjEwHwYD
VR0jBBgwFoAUYHtmGkUNl8qJUC99BM00qP/8/UswDQYJKoZIhvcNAQEFBQADggEB
AL7IG0l+k4LkcpI+a/kvZsSRwSM4uA6zGX34e78A2oytr8RG8bJwVb8+AHMUD+Xe
2kYdh/Uj/waQXfqR0OgxQXL9Ct4ZM+JlR1avsNKXWL5AwYXAXCOB3J5PW2XOck7H
Zw0vRbGQhjWjQx+B4KOUFg1b3ov/z6Xkr3yaCfRQhXh7KC0Bc0RXPPG5Nv5lCW+z
tbbg0zMm3kyfQITRusMSg6IBsDJqOnjaiaKQRcXiD0Sk43ZXb2bUKMxC7+Td3QL4
RyHcWJbQ7YylLTS/x+jxWIcOQ0oO5/54t5PTQ14neYhOz9x4gUk2AYAW6d1vePwb
hcC8roQwkHT7HvfYBoc74FM=
-----END CERTIFICATE-----`

var globalSignRoot = `-----BEGIN CERTIFICATE-----
MIIDdTCCAl2gAwIBAgILBAAAAAABFUtaw5QwDQYJKoZIhvcNAQEFBQAwVzELMAkG
A1UEBhMCQkUxGTAXBgNVBAoTEEdsb2JhbFNpZ24gbnYtc2ExEDAOBgNVBAsTB1Jv
b3QgQ0ExGzAZBgNVBAMTEkdsb2JhbFNpZ24gUm9vdCBDQTAeFw05ODA5MDExMjAw
MDBaFw0yODAxMjgxMjAwMDBaMFcxCzAJBgNVBAYTAkJFMRkwFwYDVQQKExBHbG9i
YWxTaWduIG52LXNhMRAwDgYDVQQLEwdSb290IENBMRswGQYDVQQDExJHbG9iYWxT
aWduIFJvb3QgQ0EwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDaDuaZ
jc6j40+Kfvvxi4Mla+pIH/EqsLmVEQS98GPR4mdmzxzdzxtIK+6NiY6arymAZavp
xy0Sy6scTHAHoT0KMM0VjU/43dSMUBUc71DuxC73/OlS8pF94G3VNTCOXkNz8kHp
1Wrjsok6Vjk4bwY8iGlbKk3Fp1S4bInMm/k8yuX9ifUSPJJ4ltbcdG6TRGHRjcdG
snUOhugZitVtbNV4FpWi6cgKOOvyJBNPc1STE4U6G7weNLWLBYy5d4ux2x8gkasJ
U26Qzns3dLlwR5EiUWMWea6xrkEmCMgZK9FGqkjWZCrXgzT/LCrBbBlDSgeF59N8
9iFo7+ryUp9/k5DPAgMBAAGjQjBAMA4GA1UdDwEB/wQEAwIBBjAPBgNVHRMBAf8E
BTADAQH/MB0GA1UdDgQWBBRge2YaRQ2XyolQL30EzTSo//z9SzANBgkqhkiG9w0B
AQUFAAOCAQEA1nPnfE920I2/7LqivjTFKDK1fPxsnCwrvQmeU79rXqoRSLblCKOz
yj1hTdNGCbM+w6DjY1Ub8rrvrTnhQ7k4o+YviiY776BQVvnGCv04zcQLcFGUl5gE
38NflNUVyRRBnMRddWQVDf9VMOyGj/8N7yy5Y0b2qvzfvGn9LhJIZJrglfCm7ymP
AbEVtQwdpf5pLGkkeB6zpxxxYu7KyJesF12KwvhHhm4qxFYxldBniYUr+WymXUad
DKqC5JlR3XC321Y9YeRq4VzW9v493kHMB65jUr9TU/Qr6cf9tveCX4XSQRjbgbME
HMUfpIBvFSDJ3gyICh3WZlXi/EjJKSZp4A==
-----END CERTIFICATE-----`

var moipLeafCert = `-----BEGIN CERTIFICATE-----
MIIGQDCCBSigAwIBAgIRAPe/cwh7CUWizo8mYSDavLIwDQYJKoZIhvcNAQELBQAw
gZIxCzAJBgNVBAYTAkdCMRswGQYDVQQIExJHcmVhdGVyIE1hbmNoZXN0ZXIxEDAO
BgNVBAcTB1NhbGZvcmQxGjAYBgNVBAoTEUNPTU9ETyBDQSBMaW1pdGVkMTgwNgYD
VQQDEy9DT01PRE8gUlNBIEV4dGVuZGVkIFZhbGlkYXRpb24gU2VjdXJlIFNlcnZl
ciBDQTAeFw0xMzA4MTUwMDAwMDBaFw0xNDA4MTUyMzU5NTlaMIIBQjEXMBUGA1UE
BRMOMDg3MTg0MzEwMDAxMDgxEzARBgsrBgEEAYI3PAIBAxMCQlIxGjAYBgsrBgEE
AYI3PAIBAhMJU2FvIFBhdWxvMR0wGwYDVQQPExRQcml2YXRlIE9yZ2FuaXphdGlv
bjELMAkGA1UEBhMCQlIxETAPBgNVBBETCDAxNDUyMDAwMRIwEAYDVQQIEwlTYW8g
UGF1bG8xEjAQBgNVBAcTCVNhbyBQYXVsbzEtMCsGA1UECRMkQXZlbmlkYSBCcmln
YWRlaXJvIEZhcmlhIExpbWEgLCAyOTI3MR0wGwYDVQQKExRNb2lwIFBhZ2FtZW50
b3MgUy5BLjENMAsGA1UECxMETU9JUDEYMBYGA1UECxMPU1NMIEJsaW5kYWRvIEVW
MRgwFgYDVQQDEw9hcGkubW9pcC5jb20uYnIwggEiMA0GCSqGSIb3DQEBAQUAA4IB
DwAwggEKAoIBAQDN0b9x6TrXXA9hPCF8/NjqGJ++2D4LO4ZiMFTjs0VwpXy2Y1Oe
s74/HuiLGnAHxTmAtV7IpZMibiOcTxcnDYp9oEWkf+gR+hZvwFZwyOBC7wyb3SR3
UvV0N1ZbEVRYpN9kuX/3vjDghjDmzzBwu8a/T+y5JTym5uiJlngVAWyh/RjtIvYi
+NVkQMbyVlPGkoCe6c30pH8DKYuUCZU6DHjUsPTX3jAskqbhDSAnclX9iX0p2bmw
KVBc+5Vh/2geyzDuquF0w+mNIYdU5h7uXvlmJnf3d2Cext5dxdL8/jezD3U0dAqI
pYSKERbyxSkJWxdvRlhdpM9YXMJcpc88xNp1AgMBAAGjggHcMIIB2DAfBgNVHSME
GDAWgBQ52v/KKBSKqHQTCLnkDqnS+n6daTAdBgNVHQ4EFgQU/lXuOa7DMExzZjRj
LQWcMWGZY7swDgYDVR0PAQH/BAQDAgWgMAwGA1UdEwEB/wQCMAAwHQYDVR0lBBYw
FAYIKwYBBQUHAwEGCCsGAQUFBwMCMEYGA1UdIAQ/MD0wOwYMKwYBBAGyMQECAQUB
MCswKQYIKwYBBQUHAgEWHWh0dHBzOi8vc2VjdXJlLmNvbW9kby5jb20vQ1BTMFYG
A1UdHwRPME0wS6BJoEeGRWh0dHA6Ly9jcmwuY29tb2RvY2EuY29tL0NPTU9ET1JT
QUV4dGVuZGVkVmFsaWRhdGlvblNlY3VyZVNlcnZlckNBLmNybDCBhwYIKwYBBQUH
AQEEezB5MFEGCCsGAQUFBzAChkVodHRwOi8vY3J0LmNvbW9kb2NhLmNvbS9DT01P
RE9SU0FFeHRlbmRlZFZhbGlkYXRpb25TZWN1cmVTZXJ2ZXJDQS5jcnQwJAYIKwYB
BQUHMAGGGGh0dHA6Ly9vY3NwLmNvbW9kb2NhLmNvbTAvBgNVHREEKDAmgg9hcGku
bW9pcC5jb20uYnKCE3d3dy5hcGkubW9pcC5jb20uYnIwDQYJKoZIhvcNAQELBQAD
ggEBAFoTmPlaDcf+nudhjXHwud8g7/LRyA8ucb+3/vfmgbn7FUc1eprF5sJS1mA+
pbiTyXw4IxcJq2KUj0Nw3IPOe9k84mzh+XMmdCKH+QK3NWkE9Udz+VpBOBc0dlqC
1RH5umStYDmuZg/8/r652eeQ5kUDcJyADfpKWBgDPYaGtwzKVT4h3Aok9SLXRHx6
z/gOaMjEDMarMCMw4VUIG1pvNraZrG5oTaALPaIXXpd8VqbQYPudYJ6fR5eY3FeW
H/ofbYFdRcuD26MfBFWE9VGGral9Fgo8sEHffho+UWhgApuQV4/l5fMzxB5YBXyQ
jhuy8PqqZS9OuLilTeLu4a8z2JI=
-----END CERTIFICATE-----`

var comodoIntermediateSHA384 = `-----BEGIN CERTIFICATE-----
MIIGDjCCA/agAwIBAgIQBqdDgNTr/tQ1taP34Wq92DANBgkqhkiG9w0BAQwFADCB
hTELMAkGA1UEBhMCR0IxGzAZBgNVBAgTEkdyZWF0ZXIgTWFuY2hlc3RlcjEQMA4G
A1UEBxMHU2FsZm9yZDEaMBgGA1UEChMRQ09NT0RPIENBIExpbWl0ZWQxKzApBgNV
BAMTIkNPTU9ETyBSU0EgQ2VydGlmaWNhdGlvbiBBdXRob3JpdHkwHhcNMTIwMjEy
MDAwMDAwWhcNMjcwMjExMjM1OTU5WjCBkjELMAkGA1UEBhMCR0IxGzAZBgNVBAgT
EkdyZWF0ZXIgTWFuY2hlc3RlcjEQMA4GA1UEBxMHU2FsZm9yZDEaMBgGA1UEChMR
Q09NT0RPIENBIExpbWl0ZWQxODA2BgNVBAMTL0NPTU9ETyBSU0EgRXh0ZW5kZWQg
VmFsaWRhdGlvbiBTZWN1cmUgU2VydmVyIENBMIIBIjANBgkqhkiG9w0BAQEFAAOC
AQ8AMIIBCgKCAQEAlVbeVLTf1QJJe9FbXKKyHo+cK2JMK40SKPMalaPGEP0p3uGf
CzhAk9HvbpUQ/OGQF3cs7nU+e2PsYZJuTzurgElr3wDqAwB/L3XVKC/sVmePgIOj
vdwDmZOLlJFWW6G4ajo/Br0OksxgnP214J9mMF/b5pTwlWqvyIqvgNnmiDkBfBzA
xSr3e5Wg8narbZtyOTDr0VdVAZ1YEZ18bYSPSeidCfw8/QpKdhQhXBZzQCMZdMO6
WAqmli7eNuWf0MLw4eDBYuPCGEUZUaoXHugjddTI0JYT/8ck0YwLJ66eetw6YWNg
iJctXQUL5Tvrrs46R3N2qPos3cCHF+msMJn4HwIDAQABo4IBaTCCAWUwHwYDVR0j
BBgwFoAUu69+Aj36pvE8hI6t7jiY7NkyMtQwHQYDVR0OBBYEFDna/8ooFIqodBMI
ueQOqdL6fp1pMA4GA1UdDwEB/wQEAwIBBjASBgNVHRMBAf8ECDAGAQH/AgEAMD4G
A1UdIAQ3MDUwMwYEVR0gADArMCkGCCsGAQUFBwIBFh1odHRwczovL3NlY3VyZS5j
b21vZG8uY29tL0NQUzBMBgNVHR8ERTBDMEGgP6A9hjtodHRwOi8vY3JsLmNvbW9k
b2NhLmNvbS9DT01PRE9SU0FDZXJ0aWZpY2F0aW9uQXV0aG9yaXR5LmNybDBxBggr
BgEFBQcBAQRlMGMwOwYIKwYBBQUHMAKGL2h0dHA6Ly9jcnQuY29tb2RvY2EuY29t
L0NPTU9ET1JTQUFkZFRydXN0Q0EuY3J0MCQGCCsGAQUFBzABhhhodHRwOi8vb2Nz
cC5jb21vZG9jYS5jb20wDQYJKoZIhvcNAQEMBQADggIBAERCnUFRK0iIXZebeV4R
AUpSGXtBLMeJPNBy3IX6WK/VJeQT+FhlZ58N/1eLqYVeyqZLsKeyLeCMIs37/3mk
jCuN/gI9JN6pXV/kD0fQ22YlPodHDK4ixVAihNftSlka9pOlk7DgG4HyVsTIEFPk
1Hax0VtpS3ey4E/EhOfUoFDuPPpE/NBXueEoU/1Tzdy5H3pAvTA/2GzS8+cHnx8i
teoiccsq8FZ8/qyo0QYPFBRSTP5kKwxpKrgNUG4+BAe/eiCL+O5lCeHHSQgyPQ0o
fkkdt0rvAucNgBfIXOBhYsvss2B5JdoaZXOcOBCgJjqwyBZ9kzEi7nQLiMBciUEA
KKlHMd99SUWa9eanRRrSjhMQ34Ovmw2tfn6dNVA0BM7pINae253UqNpktNEvWS5e
ojZh1CSggjMziqHRbO9haKPl0latxf1eYusVqHQSTC8xjOnB3xBLAer2VBvNfzu9
XJ/B288ByvK6YBIhMe2pZLiySVgXbVrXzYxtvp5/4gJYp9vDLVj2dAZqmvZh+fYA
tmnYOosxWd2R5nwnI4fdAw+PKowegwFOAWEMUnNt/AiiuSpm5HZNMaBWm9lTjaK2
jwLI5jqmBNFI+8NKAnb9L9K8E7bobTQk+p0pisehKxTxlgBzuRPpwLk6R1YCcYAn
pLwltum95OmYdBbxN4SBB7SC
-----END CERTIFICATE-----`

const comodoRSAAuthority = `-----BEGIN CERTIFICATE-----
MIIFdDCCBFygAwIBAgIQJ2buVutJ846r13Ci/ITeIjANBgkqhkiG9w0BAQwFADBv
MQswCQYDVQQGEwJTRTEUMBIGA1UEChMLQWRkVHJ1c3QgQUIxJjAkBgNVBAsTHUFk
ZFRydXN0IEV4dGVybmFsIFRUUCBOZXR3b3JrMSIwIAYDVQQDExlBZGRUcnVzdCBF
eHRlcm5hbCBDQSBSb290MB4XDTAwMDUzMDEwNDgzOFoXDTIwMDUzMDEwNDgzOFow
gYUxCzAJBgNVBAYTAkdCMRswGQYDVQQIExJHcmVhdGVyIE1hbmNoZXN0ZXIxEDAO
BgNVBAcTB1NhbGZvcmQxGjAYBgNVBAoTEUNPTU9ETyBDQSBMaW1pdGVkMSswKQYD
VQQDEyJDT01PRE8gUlNBIENlcnRpZmljYXRpb24gQXV0aG9yaXR5MIICIjANBgkq
hkiG9w0BAQEFAAOCAg8AMIICCgKCAgEAkehUktIKVrGsDSTdxc9EZ3SZKzejfSNw
AHG8U9/E+ioSj0t/EFa9n3Byt2F/yUsPF6c947AEYe7/EZfH9IY+Cvo+XPmT5jR6
2RRr55yzhaCCenavcZDX7P0N+pxs+t+wgvQUfvm+xKYvT3+Zf7X8Z0NyvQwA1onr
ayzT7Y+YHBSrfuXjbvzYqOSSJNpDa2K4Vf3qwbxstovzDo2a5JtsaZn4eEgwRdWt
4Q08RWD8MpZRJ7xnw8outmvqRsfHIKCxH2XeSAi6pE6p8oNGN4Tr6MyBSENnTnIq
m1y9TBsoilwie7SrmNnu4FGDwwlGTm0+mfqVF9p8M1dBPI1R7Qu2XK8sYxrfV8g/
vOldxJuvRZnio1oktLqpVj3Pb6r/SVi+8Kj/9Lit6Tf7urj0Czr56ENCHonYhMsT
8dm74YlguIwoVqwUHZwK53Hrzw7dPamWoUi9PPevtQ0iTMARgexWO/bTouJbt7IE
IlKVgJNp6I5MZfGRAy1wdALqi2cVKWlSArvX31BqVUa/oKMoYX9w0MOiqiwhqkfO
KJwGRXa/ghgntNWutMtQ5mv0TIZxMOmm3xaG4Nj/QN370EKIf6MzOi5cHkERgWPO
GHFrK+ymircxXDpqR+DDeVnWIBqv8mqYqnK8V0rSS527EPywTEHl7R09XiidnMy/
s1Hap0flhFMCAwEAAaOB9DCB8TAfBgNVHSMEGDAWgBStvZh6NLQm9/rEJlTvA73g
JMtUGjAdBgNVHQ4EFgQUu69+Aj36pvE8hI6t7jiY7NkyMtQwDgYDVR0PAQH/BAQD
AgGGMA8GA1UdEwEB/wQFMAMBAf8wEQYDVR0gBAowCDAGBgRVHSAAMEQGA1UdHwQ9
MDswOaA3oDWGM2h0dHA6Ly9jcmwudXNlcnRydXN0LmNvbS9BZGRUcnVzdEV4dGVy
bmFsQ0FSb290LmNybDA1BggrBgEFBQcBAQQpMCcwJQYIKwYBBQUHMAGGGWh0dHA6
Ly9vY3NwLnVzZXJ0cnVzdC5jb20wDQYJKoZIhvcNAQEMBQADggEBAGS/g/FfmoXQ
zbihKVcN6Fr30ek+8nYEbvFScLsePP9NDXRqzIGCJdPDoCpdTPW6i6FtxFQJdcfj
Jw5dhHk3QBN39bSsHNA7qxcS1u80GH4r6XnTq1dFDK8o+tDb5VCViLvfhVdpfZLY
Uspzgb8c8+a4bmYRBbMelC1/kZWSWfFMzqORcUx8Rww7Cxn2obFshj5cqsQugsv5
B5a6SE2Q8pTIqXOi6wZ7I53eovNNVZ96YUWYGGjHXkBrI/V5eu+MtWuLt29G9Hvx
PUsE2JOAWVrgQSQdso8VYFhH2+9uRv0V9dlfmrPb2LjkQLPNlzmuhbsdjrzch5vR
pu/xO28QOG8=
-----END CERTIFICATE-----`

const addTrustRoot = `-----BEGIN CERTIFICATE-----
MIIENjCCAx6gAwIBAgIBATANBgkqhkiG9w0BAQUFADBvMQswCQYDVQQGEwJTRTEU
MBIGA1UEChMLQWRkVHJ1c3QgQUIxJjAkBgNVBAsTHUFkZFRydXN0IEV4dGVybmFs
IFRUUCBOZXR3b3JrMSIwIAYDVQQDExlBZGRUcnVzdCBFeHRlcm5hbCBDQSBSb290
MB4XDTAwMDUzMDEwNDgzOFoXDTIwMDUzMDEwNDgzOFowbzELMAkGA1UEBhMCU0Ux
FDASBgNVBAoTC0FkZFRydXN0IEFCMSYwJAYDVQQLEx1BZGRUcnVzdCBFeHRlcm5h
bCBUVFAgTmV0d29yazEiMCAGA1UEAxMZQWRkVHJ1c3QgRXh0ZXJuYWwgQ0EgUm9v
dDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBALf3GjPm8gAELTngTlvt
H7xsD821+iO2zt6bETOXpClMfZOfvUq8k+0DGuOPz+VtUFrWlymUWoCwSXrbLpX9
uMq/NzgtHj6RQa1wVsfwTz/oMp50ysiQVOnGXw94nZpAPA6sYapeFI+eh6FqUNzX
mk6vBbOmcZSccbNQYArHE504B4YCqOmoaSYYkKtMsE8jqzpPhNjfzp/haW+710LX
a0Tkx63ubUFfclpxCDezeWWkWaCUN/cALw3CknLa0Dhy2xSoRcRdKn23tNbE7qzN
E0S3ySvdQwAl+mG5aWpYIxG3pzOPVnVZ9c0p10a3CitlttNCbxWyuHv77+ldU9U0
WicCAwEAAaOB3DCB2TAdBgNVHQ4EFgQUrb2YejS0Jvf6xCZU7wO94CTLVBowCwYD
VR0PBAQDAgEGMA8GA1UdEwEB/wQFMAMBAf8wgZkGA1UdIwSBkTCBjoAUrb2YejS0
Jvf6xCZU7wO94CTLVBqhc6RxMG8xCzAJBgNVBAYTAlNFMRQwEgYDVQQKEwtBZGRU
cnVzdCBBQjEmMCQGA1UECxMdQWRkVHJ1c3QgRXh0ZXJuYWwgVFRQIE5ldHdvcmsx
IjAgBgNVBAMTGUFkZFRydXN0IEV4dGVybmFsIENBIFJvb3SCAQEwDQYJKoZIhvcN
AQEFBQADggEBALCb4IUlwtYj4g+WBpKdQZic2YR5gdkeWxQHIzZlj7DYd7usQWxH
YINRsPkyPef89iYTx4AWpb9a/IfPeHmJIZriTAcKhjW88t5RxNKWt9x+Tu5w/Rw5
6wwCURQtjr0W4MHfRnXnJK3s9EK0hZNwEGe6nQY1ShjTK3rMUUKhemPR5ruhxSvC
Nr4TDea9Y355e6cJDUCrat2PisP29owaQgVR1EX1n6diIWgVIEM8med8vSTYqZEX
c4g/VhsxOBi0cQ+azcgOno4uG+GMmIPLHzHxREzGBHNJdmAPx/i9F4BrLunMTA5a
mnkPIAou1Z5jJh5VkpTYghdae9C8x49OhgQ=
-----END CERTIFICATE-----`

const zcryptoRoot = `
Certificate:
    Data:
        Version: 3 (0x2)
        Serial Number: 0 (0x0)
        Signature Algorithm: sha256WithRSAEncryption
        Issuer: CN=ZCrypto Root Authority
        Validity
            Not Before: Jan  1 00:00:00 2017 GMT
            Not After : Jan  1 00:00:00 2027 GMT
        Subject: CN=ZCrypto Root Authority
        Subject Public Key Info:
            Public Key Algorithm: rsaEncryption
            RSA Public Key: (2048 bit)
                Modulus (2048 bit):
                    00:cd:f3:5a:43:f9:8a:16:e8:73:2e:3d:76:39:49:
                    bb:bf:9e:a1:58:23:8a:d0:ea:55:92:6e:d5:4f:d3:
                    be:32:ea:5d:66:e9:ae:0f:e0:b7:2e:2e:97:90:f6:
                    fb:c1:0b:73:e3:bc:d8:7b:93:35:ac:f4:01:f9:bc:
                    ca:97:a0:ce:8c:eb:f6:73:9b:63:a9:ae:09:fb:50:
                    1b:c0:5f:3e:ac:93:d6:8c:e2:97:28:af:d2:4a:00:
                    85:1e:81:ec:ce:0f:5d:b1:39:08:e9:0c:fa:07:3c:
                    85:af:09:e4:09:c8:7a:8f:9d:58:cc:a5:28:ba:d1:
                    d8:3a:0a:cd:b5:0d:0b:26:71:16:60:86:a4:87:b5:
                    b7:48:a5:21:27:44:b1:bf:06:7e:a9:74:17:69:b7:
                    55:0d:75:bb:a6:cd:e5:40:a2:4a:f9:17:3b:cf:e3:
                    0e:0a:d9:2b:09:43:f5:8a:f9:c8:5b:6b:1d:0e:b1:
                    61:d6:12:2c:04:ba:65:72:7a:b0:ff:a0:0d:03:28:
                    fd:5e:bc:a9:06:b4:cf:34:81:40:af:e4:72:90:4e:
                    4a:37:89:54:c9:cf:5b:1f:e4:4f:1f:15:4a:54:f3:
                    9d:2a:8c:dd:9e:58:59:38:68:d8:27:2b:7c:08:82:
                    60:3e:f1:0f:f7:8f:d5:87:6e:11:ec:44:f2:e4:ee:
                    e7:f3
                Exponent: 65537 (0x10001)
        X509v3 extensions:
            X509v3 Basic Constraints: critical
                CA:TRUE
            X509v3 Subject Key Identifier:
                F7:7D:D8:A5:24:B8:D0:D1:A0:14:7C:5D:91:67:38:8A:EB:0D:31:95
            X509v3 Authority Key Identifier:
                keyid:F7:7D:D8:A5:24:B8:D0:D1:A0:14:7C:5D:91:67:38:8A:EB:0D:31:95

            X509v3 Key Usage: critical
                Certificate Sign, CRL Sign
    Signature Algorithm: sha256WithRSAEncryption
        59:18:0c:0c:fc:70:f0:f9:38:6d:70:71:69:a5:6a:19:96:b7:
        56:fa:d1:a7:56:04:12:77:36:05:a9:a2:d5:97:1e:8e:ce:78:
        b9:72:09:bf:73:4f:32:90:0f:a0:6a:77:40:8c:00:92:4e:c9:
        a7:78:05:4f:b5:42:c6:f3:b2:d9:02:ec:a1:98:e9:2b:3a:2f:
        67:02:3f:44:f2:bb:f3:7d:ee:4e:6c:2f:3f:29:56:6d:aa:bc:
        fc:8c:c0:3d:7c:bf:96:01:87:3f:e1:3b:71:3b:ef:26:25:3e:
        61:90:90:d0:02:31:f1:64:ae:bc:bb:62:04:05:10:5e:fe:dd:
        34:92:d7:2c:32:1f:b7:30:d6:b7:57:8b:b8:b4:29:9c:e5:bb:
        c6:5b:6d:f5:5c:80:e1:c8:c4:30:eb:eb:44:45:86:bf:71:08:
        b7:e3:79:10:02:08:70:66:58:a1:21:b7:7e:01:9d:3b:ea:6d:
        a6:68:c9:12:6e:f4:c5:ea:b8:1d:38:27:49:b6:60:ac:8f:5a:
        f3:9f:27:28:e2:4c:87:a7:cf:f9:f8:72:5b:b4:4e:98:bf:5b:
        fb:cd:52:6e:6b:56:d7:dd:32:2f:28:d9:30:cb:dd:40:3b:79:
        50:7c:97:70:ce:b3:c2:57:4d:e0:9b:05:89:43:2a:3d:78:bc:
        e8:95:a5:14
-----BEGIN CERTIFICATE-----
MIIDIDCCAgigAwIBAgIBADANBgkqhkiG9w0BAQsFADAhMR8wHQYDVQQDDBZaQ3J5
cHRvIFJvb3QgQXV0aG9yaXR5MB4XDTE3MDEwMTAwMDAwMFoXDTI3MDEwMTAwMDAw
MFowITEfMB0GA1UEAwwWWkNyeXB0byBSb290IEF1dGhvcml0eTCCASIwDQYJKoZI
hvcNAQEBBQADggEPADCCAQoCggEBAM3zWkP5ihbocy49djlJu7+eoVgjitDqVZJu
1U/TvjLqXWbprg/gty4ul5D2+8ELc+O82HuTNaz0Afm8ypegzozr9nObY6muCftQ
G8BfPqyT1ozilyiv0koAhR6B7M4PXbE5COkM+gc8ha8J5AnIeo+dWMylKLrR2DoK
zbUNCyZxFmCGpIe1t0ilISdEsb8Gfql0F2m3VQ11u6bN5UCiSvkXO8/jDgrZKwlD
9Yr5yFtrHQ6xYdYSLAS6ZXJ6sP+gDQMo/V68qQa0zzSBQK/kcpBOSjeJVMnPWx/k
Tx8VSlTznSqM3Z5YWTho2CcrfAiCYD7xD/eP1YduEexE8uTu5/MCAwEAAaNjMGEw
DwYDVR0TAQH/BAUwAwEB/zAdBgNVHQ4EFgQU933YpSS40NGgFHxdkWc4iusNMZUw
HwYDVR0jBBgwFoAU933YpSS40NGgFHxdkWc4iusNMZUwDgYDVR0PAQH/BAQDAgEG
MA0GCSqGSIb3DQEBCwUAA4IBAQBZGAwM/HDw+ThtcHFppWoZlrdW+tGnVgQSdzYF
qaLVlx6Ozni5cgm/c08ykA+gandAjACSTsmneAVPtULG87LZAuyhmOkrOi9nAj9E
8rvzfe5ObC8/KVZtqrz8jMA9fL+WAYc/4TtxO+8mJT5hkJDQAjHxZK68u2IEBRBe
/t00ktcsMh+3MNa3V4u4tCmc5bvGW231XIDhyMQw6+tERYa/cQi343kQAghwZlih
Ibd+AZ076m2maMkSbvTF6rgdOCdJtmCsj1rznyco4kyHp8/5+HJbtE6Yv1v7zVJu
a1bX3TIvKNkwy91AO3lQfJdwzrPCV03gmwWJQyo9eLzolaUU
-----END CERTIFICATE-----
`
const zcryptoIntermediate = `
Certificate:
    Data:
        Version: 3 (0x2)
        Serial Number: 1 (0x1)
        Signature Algorithm: sha256WithRSAEncryption
        Issuer: CN=ZCrypto Root Authority
        Validity
            Not Before: Jan  1 00:00:00 2020 GMT
            Not After : Dec 31 00:00:00 2026 GMT
        Subject: CN=ZCrypto Intermediate Authority
        Subject Public Key Info:
            Public Key Algorithm: rsaEncryption
            RSA Public Key: (2048 bit)
                Modulus (2048 bit):
                    00:cb:63:b2:bd:28:93:52:78:aa:88:a7:9f:0f:92:
                    8b:53:fe:07:61:e1:c1:c2:0e:67:b7:bb:61:e8:53:
                    c2:7f:cb:41:a0:2d:7f:46:a8:1a:65:8f:5b:96:69:
                    39:15:97:db:04:84:7e:37:56:5e:1d:50:8f:16:2e:
                    97:e4:25:b7:3d:1a:a6:7a:0c:3b:1c:1d:0c:e6:61:
                    ff:d4:51:e0:60:22:0a:d7:5d:34:09:80:fc:67:fd:
                    33:3b:ef:e1:04:6e:f4:2c:f9:2a:81:8f:5e:88:a7:
                    9a:b6:18:87:d9:aa:8b:8b:5a:ef:bc:bd:ba:08:55:
                    39:20:0e:b2:8e:6b:7a:b9:b7:97:2c:d5:ab:7e:fc:
                    41:7c:ce:4b:1d:ec:8b:0c:6b:2f:e4:ac:0e:44:79:
                    59:7f:21:cc:ce:a9:f1:95:9c:1a:b6:7b:94:16:a9:
                    ba:ed:b0:48:8e:ec:25:c0:db:68:66:cb:3b:de:27:
                    6d:c0:d2:02:9b:e3:9e:38:2d:bd:ea:90:00:1d:f3:
                    e8:8e:6d:6d:43:9a:68:07:6a:7a:eb:0d:71:0b:02:
                    83:61:56:cf:d2:d0:dd:9e:b2:0e:74:39:26:bc:e0:
                    3b:24:d4:6a:99:33:15:b8:37:c8:ae:97:c1:78:74:
                    6b:26:67:d1:91:74:c4:91:90:d3:96:ad:87:f9:19:
                    67:63
                Exponent: 65537 (0x10001)
        X509v3 extensions:
            X509v3 Basic Constraints: critical
                CA:TRUE
            X509v3 Subject Key Identifier:
                23:56:59:53:5A:3D:D7:DF:2E:F3:A8:68:C0:12:77:FB:55:D8:50:EA
            X509v3 Authority Key Identifier:
                keyid:F7:7D:D8:A5:24:B8:D0:D1:A0:14:7C:5D:91:67:38:8A:EB:0D:31:95

            X509v3 Key Usage: critical
                Certificate Sign, CRL Sign
    Signature Algorithm: sha256WithRSAEncryption
        68:59:cf:9d:2e:7e:19:61:63:4d:d2:61:c0:98:0c:0d:70:2f:
        cb:1c:1e:b7:27:a0:07:e5:13:92:5c:ff:2f:93:ca:04:a1:a9:
        99:e0:3d:92:b3:63:a9:92:67:ab:e2:94:b8:b0:7e:cd:78:eb:
        e6:68:65:b1:53:2a:c5:e2:76:f9:ce:ee:fe:f4:46:85:51:3e:
        20:f2:9b:ce:9f:ef:03:f6:22:c1:31:d3:0e:f4:bb:cd:8f:88:
        7d:24:2d:9b:93:b1:28:c7:20:11:7b:cf:67:4d:c0:81:f1:b0:
        92:36:34:e5:a9:f2:e0:cf:cf:6b:d5:8c:07:d3:c1:66:b0:32:
        b0:2f:ec:11:70:7b:d0:63:a8:db:bc:fa:d9:93:67:7e:3b:9c:
        95:ca:4b:a0:14:fd:f2:80:fb:04:2e:86:3c:75:89:31:21:0b:
        9a:13:5b:56:f0:c1:ea:0f:6c:8f:ec:9a:5c:b8:8a:75:5e:c3:
        b2:00:b2:3d:37:75:0e:fa:43:43:7e:4d:fd:8e:95:71:7a:0a:
        e6:2a:a1:59:bb:45:65:5c:3e:d4:d6:f1:5e:27:e3:18:7e:1b:
        59:cc:46:71:7c:32:fc:2a:b8:a0:02:29:99:8f:be:62:95:9a:
        d0:b2:d6:03:36:82:54:fc:4b:69:e4:c6:fd:b3:c4:e4:5b:55:
        14:a8:c6:4f
-----BEGIN CERTIFICATE-----
MIIDKDCCAhCgAwIBAgIBATANBgkqhkiG9w0BAQsFADAhMR8wHQYDVQQDDBZaQ3J5
cHRvIFJvb3QgQXV0aG9yaXR5MB4XDTIwMDEwMTAwMDAwMFoXDTI2MTIzMTAwMDAw
MFowKTEnMCUGA1UEAwweWkNyeXB0byBJbnRlcm1lZGlhdGUgQXV0aG9yaXR5MIIB
IjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAy2OyvSiTUniqiKefD5KLU/4H
YeHBwg5nt7th6FPCf8tBoC1/RqgaZY9blmk5FZfbBIR+N1ZeHVCPFi6X5CW3PRqm
egw7HB0M5mH/1FHgYCIK1100CYD8Z/0zO+/hBG70LPkqgY9eiKeathiH2aqLi1rv
vL26CFU5IA6yjmt6ubeXLNWrfvxBfM5LHeyLDGsv5KwORHlZfyHMzqnxlZwatnuU
Fqm67bBIjuwlwNtoZss73idtwNICm+OeOC296pAAHfPojm1tQ5poB2p66w1xCwKD
YVbP0tDdnrIOdDkmvOA7JNRqmTMVuDfIrpfBeHRrJmfRkXTEkZDTlq2H+RlnYwID
AQABo2MwYTAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBQjVllTWj3X3y7zqGjA
Enf7VdhQ6jAfBgNVHSMEGDAWgBT3fdilJLjQ0aAUfF2RZziK6w0xlTAOBgNVHQ8B
Af8EBAMCAQYwDQYJKoZIhvcNAQELBQADggEBAGhZz50ufhlhY03SYcCYDA1wL8sc
HrcnoAflE5Jc/y+TygShqZngPZKzY6mSZ6vilLiwfs146+ZoZbFTKsXidvnO7v70
RoVRPiDym86f7wP2IsEx0w70u82PiH0kLZuTsSjHIBF7z2dNwIHxsJI2NOWp8uDP
z2vVjAfTwWawMrAv7BFwe9BjqNu8+tmTZ347nJXKS6AU/fKA+wQuhjx1iTEhC5oT
W1bwweoPbI/smly4inVew7IAsj03dQ76Q0N+Tf2OlXF6CuYqoVm7RWVcPtTW8V4n
4xh+G1nMRnF8MvwquKACKZmPvmKVmtCy1gM2glT8S2nkxv2zxORbVRSoxk8=
-----END CERTIFICATE-----
`
const zcryptoNeverValid = `
Certificate:
    Data:
        Version: 3 (0x2)
        Serial Number: 255 (0xff)
        Signature Algorithm: sha256WithRSAEncryption
        Issuer: CN=ZCrypto Intermediate Authority
        Validity
            Not Before: Jan  1 01:00:00 2022 GMT
            Not After : Jan  1 01:00:00 2021 GMT
        Subject: CN=never-valid.example.com
        Subject Public Key Info:
            Public Key Algorithm: rsaEncryption
            RSA Public Key: (2048 bit)
                Modulus (2048 bit):
                    00:c8:b5:df:e8:39:ba:15:5c:7c:a3:e4:97:ae:9e:
                    80:c9:9c:00:38:b0:05:75:56:dc:54:11:ca:f1:6d:
                    d5:4c:ad:0a:92:b7:b9:ed:00:6a:bf:84:e2:8a:71:
                    a2:31:a4:ee:4e:0a:7e:61:6d:6c:8e:45:36:51:b8:
                    66:dc:5a:0f:b1:62:36:7e:2e:05:b5:07:1b:e0:94:
                    ef:f5:3d:99:54:c0:56:d5:3b:f0:d5:95:8c:5a:74:
                    fc:3b:57:4d:b3:0c:5a:a4:5a:95:45:14:cb:cf:5b:
                    32:3c:79:08:f0:13:06:f3:0c:6e:b4:41:a9:e3:7a:
                    c2:92:ba:b0:cb:c4:0e:12:25:a2:46:fc:95:c3:0d:
                    9d:25:1e:14:6d:77:06:01:82:5b:27:21:5b:97:68:
                    1a:db:9d:ca:2d:1b:0a:bf:d7:82:3a:7d:ae:b0:cc:
                    a9:0f:f3:fc:3e:d3:e7:43:1a:67:2a:5c:65:f2:f0:
                    bb:f7:7f:ac:d6:0a:4c:78:b0:ef:78:ad:f6:69:97:
                    4f:2c:90:b4:b3:40:56:1e:fe:d9:19:80:0e:5b:43:
                    d1:21:db:a6:aa:e2:42:93:a9:d7:b1:6b:f5:f5:a8:
                    22:05:48:eb:52:0d:90:ac:03:34:ba:a3:75:32:d2:
                    f0:d8:41:70:c2:63:c9:c7:e4:30:db:75:f2:1e:b4:
                    90:51
                Exponent: 65537 (0x10001)
        X509v3 extensions:
            X509v3 Basic Constraints: critical
                CA:FALSE
            X509v3 Subject Key Identifier:
                D1:8B:87:FE:65:90:77:5C:F0:BC:FE:26:B9:AC:76:A1:91:D3:12:F0
            X509v3 Authority Key Identifier:
                keyid:23:56:59:53:5A:3D:D7:DF:2E:F3:A8:68:C0:12:77:FB:55:D8:50:EA

            X509v3 Key Usage:
                Digital Signature, Key Encipherment, Data Encipherment
    Signature Algorithm: sha256WithRSAEncryption
        b3:58:c2:51:12:c0:29:df:1b:57:5e:19:d2:b5:c7:8b:e1:39:
        f8:a5:c0:01:97:7f:35:34:6f:6a:7a:09:39:60:53:22:ee:07:
        37:c4:42:f0:a6:5c:71:6c:c6:b4:66:78:b4:e8:0e:0b:44:56:
        8a:4a:c6:2e:3c:97:89:a6:6a:2e:57:73:09:7c:01:d2:a4:80:
        7e:3e:cc:5d:5f:7c:cd:df:1e:a5:b9:0d:ae:70:6f:5d:dd:c9:
        b1:35:85:98:54:21:c4:31:d1:9c:07:b2:72:66:86:d0:aa:22:
        bc:16:5f:fc:b8:eb:cd:fe:01:c5:65:64:f7:b9:bb:14:69:7e:
        52:12:1e:cc:9e:93:c4:07:0a:2f:6e:03:de:2e:a1:cc:30:fd:
        a6:b0:f5:9f:a3:2a:d4:97:7b:6c:84:c2:1d:93:52:cb:0a:bd:
        09:27:6a:3c:bb:50:ba:95:39:5d:99:a7:61:d4:c5:67:6b:f7:
        69:27:c0:66:7e:ba:88:ee:98:c5:91:69:ba:5c:42:14:f8:16:
        02:d8:20:d4:ea:a8:ed:e3:ac:50:d4:72:20:a8:ba:00:5f:78:
        07:8e:7b:1f:39:5d:59:3f:07:52:7c:87:7a:83:30:0f:cb:ea:
        d0:7f:4b:d0:0d:e8:21:62:43:97:22:89:b7:c9:47:b8:2e:47:
        b3:8a:93:c0
-----BEGIN CERTIFICATE-----
MIIDJDCCAgygAwIBAgICAP8wDQYJKoZIhvcNAQELBQAwKTEnMCUGA1UEAwweWkNy
eXB0byBJbnRlcm1lZGlhdGUgQXV0aG9yaXR5MB4XDTIyMDEwMTAxMDAwMFoXDTIx
MDEwMTAxMDAwMFowIjEgMB4GA1UEAwwXbmV2ZXItdmFsaWQuZXhhbXBsZS5jb20w
ggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDItd/oOboVXHyj5JeunoDJ
nAA4sAV1VtxUEcrxbdVMrQqSt7ntAGq/hOKKcaIxpO5OCn5hbWyORTZRuGbcWg+x
YjZ+LgW1BxvglO/1PZlUwFbVO/DVlYxadPw7V02zDFqkWpVFFMvPWzI8eQjwEwbz
DG60QanjesKSurDLxA4SJaJG/JXDDZ0lHhRtdwYBglsnIVuXaBrbncotGwq/14I6
fa6wzKkP8/w+0+dDGmcqXGXy8Lv3f6zWCkx4sO94rfZpl08skLSzQFYe/tkZgA5b
Q9Eh26aq4kKTqdexa/X1qCIFSOtSDZCsAzS6o3Uy0vDYQXDCY8nH5DDbdfIetJBR
AgMBAAGjXTBbMAwGA1UdEwEB/wQCMAAwHQYDVR0OBBYEFNGLh/5lkHdc8Lz+Jrms
dqGR0xLwMB8GA1UdIwQYMBaAFCNWWVNaPdffLvOoaMASd/tV2FDqMAsGA1UdDwQE
AwIEsDANBgkqhkiG9w0BAQsFAAOCAQEAs1jCURLAKd8bV14Z0rXHi+E5+KXAAZd/
NTRvanoJOWBTIu4HN8RC8KZccWzGtGZ4tOgOC0RWikrGLjyXiaZqLldzCXwB0qSA
fj7MXV98zd8epbkNrnBvXd3JsTWFmFQhxDHRnAeycmaG0KoivBZf/Ljrzf4BxWVk
97m7FGl+UhIezJ6TxAcKL24D3i6hzDD9prD1n6Mq1Jd7bITCHZNSywq9CSdqPLtQ
upU5XZmnYdTFZ2v3aSfAZn66iO6YxZFpulxCFPgWAtgg1Oqo7eOsUNRyIKi6AF94
B457HzldWT8HUnyHeoMwD8vq0H9L0A3oIWJDlyKJt8lHuC5Hs4qTwA==
-----END CERTIFICATE-----
`
const zcryptoValidBeforeIntermediate = `
Certificate:
    Data:
        Version: 3 (0x2)
        Serial Number: 256 (0x100)
        Signature Algorithm: sha256WithRSAEncryption
        Issuer: CN=ZCrypto Intermediate Authority
        Validity
            Not Before: Jan  1 01:00:00 2018 GMT
            Not After : Jan  1 01:00:00 2019 GMT
        Subject: CN=never-valid.example.com
        Subject Public Key Info:
            Public Key Algorithm: rsaEncryption
            RSA Public Key: (2048 bit)
                Modulus (2048 bit):
                    00:cf:6a:62:38:10:e0:90:4c:63:f7:b0:91:20:4b:
                    e5:ca:f2:1b:0d:44:e3:c6:cb:7c:7d:d4:73:4c:b4:
                    45:ae:3a:64:89:20:30:4b:93:20:6d:9d:40:9b:53:
                    30:b7:71:58:9f:51:5e:91:73:07:bf:8a:8a:ab:c9:
                    dc:42:7c:7e:33:9a:08:81:00:5c:06:61:01:cb:bf:
                    cd:ff:b8:fc:9e:94:83:1a:1e:05:f8:99:c3:02:5f:
                    fb:bd:53:c1:97:0b:28:4d:1a:4a:5d:00:6b:40:7a:
                    c3:e4:7e:06:e9:6b:39:de:c7:93:5e:de:b5:f2:5b:
                    a4:78:ff:22:bb:4a:b1:af:c0:3a:21:19:de:ff:c8:
                    b5:ae:cc:63:bd:60:ce:6f:ec:c3:c9:1d:aa:24:a1:
                    73:b2:d7:79:55:51:6a:92:21:aa:70:58:c4:80:df:
                    8b:fa:25:7c:8f:a0:de:03:36:3e:e1:c8:05:57:fd:
                    fa:9f:38:f3:3a:7b:fc:83:83:2d:40:da:4f:14:8d:
                    60:90:f1:d4:95:eb:09:e3:4e:4c:f6:6a:be:dc:46:
                    87:da:cd:3c:65:33:5e:a2:be:e5:4c:33:d6:e1:08:
                    30:c6:bd:35:8c:bb:1a:30:9a:e9:7e:76:e7:84:b2:
                    7c:d8:fb:27:67:6c:00:9f:6c:9e:0a:f1:db:bb:47:
                    ee:11
                Exponent: 65537 (0x10001)
        X509v3 extensions:
            X509v3 Basic Constraints: critical
                CA:FALSE
            X509v3 Subject Key Identifier:
                6D:5F:14:AB:EB:ED:2B:CE:B9:56:DF:46:86:97:3F:35:E2:6F:D9:8F
            X509v3 Authority Key Identifier:
                keyid:23:56:59:53:5A:3D:D7:DF:2E:F3:A8:68:C0:12:77:FB:55:D8:50:EA

            X509v3 Key Usage:
                Digital Signature, Key Encipherment, Data Encipherment
    Signature Algorithm: sha256WithRSAEncryption
        b1:60:2b:82:19:98:29:98:bf:4e:6c:9e:07:7e:9e:8b:13:cf:
        be:be:88:3c:be:68:65:63:7f:8a:c7:2a:9a:5f:52:12:dc:ee:
        e1:64:94:59:3e:14:c7:7d:80:82:8b:c5:f4:72:41:ad:a0:20:
        7f:d5:04:7c:66:f5:01:9e:d7:95:a4:d9:bd:f2:ad:b6:24:fb:
        e4:d4:be:5a:0d:a8:5f:b8:de:95:c5:6e:1c:f5:fc:d8:9c:e0:
        fa:5b:93:48:70:d3:98:1f:02:09:86:fe:a5:d5:d9:de:7f:c2:
        23:9c:e8:3e:f2:85:da:e5:91:2f:b0:a3:da:ba:f4:8d:88:4a:
        3d:c7:94:e7:00:cf:d9:e3:6e:19:ef:6a:09:58:27:60:49:6a:
        a9:03:98:83:88:33:ac:24:ba:63:37:73:6f:2f:10:df:06:79:
        d8:29:01:62:a5:89:4b:25:24:12:9c:6b:0c:f0:95:45:85:fe:
        1b:82:13:76:ae:c2:80:77:d8:41:60:3b:8a:7f:40:43:e5:66:
        f9:b2:f3:e8:e0:5f:0a:ec:0a:3c:22:cf:4a:2d:e7:59:0c:82:
        0d:e7:8a:4b:31:f2:04:a8:79:9f:a9:af:57:c6:6e:d7:32:ec:
        23:74:aa:e2:59:2a:88:c2:e4:0c:f6:fb:67:31:dd:44:ed:a9:
        e0:fe:2f:65
-----BEGIN CERTIFICATE-----
MIIDJDCCAgygAwIBAgICAQAwDQYJKoZIhvcNAQELBQAwKTEnMCUGA1UEAwweWkNy
eXB0byBJbnRlcm1lZGlhdGUgQXV0aG9yaXR5MB4XDTE4MDEwMTAxMDAwMFoXDTE5
MDEwMTAxMDAwMFowIjEgMB4GA1UEAwwXbmV2ZXItdmFsaWQuZXhhbXBsZS5jb20w
ggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDPamI4EOCQTGP3sJEgS+XK
8hsNROPGy3x91HNMtEWuOmSJIDBLkyBtnUCbUzC3cVifUV6Rcwe/ioqrydxCfH4z
mgiBAFwGYQHLv83/uPyelIMaHgX4mcMCX/u9U8GXCyhNGkpdAGtAesPkfgbpazne
x5Ne3rXyW6R4/yK7SrGvwDohGd7/yLWuzGO9YM5v7MPJHaokoXOy13lVUWqSIapw
WMSA34v6JXyPoN4DNj7hyAVX/fqfOPM6e/yDgy1A2k8UjWCQ8dSV6wnjTkz2ar7c
RofazTxlM16ivuVMM9bhCDDGvTWMuxowmul+dueEsnzY+ydnbACfbJ4K8du7R+4R
AgMBAAGjXTBbMAwGA1UdEwEB/wQCMAAwHQYDVR0OBBYEFG1fFKvr7SvOuVbfRoaX
PzXib9mPMB8GA1UdIwQYMBaAFCNWWVNaPdffLvOoaMASd/tV2FDqMAsGA1UdDwQE
AwIEsDANBgkqhkiG9w0BAQsFAAOCAQEAsWArghmYKZi/TmyeB36eixPPvr6IPL5o
ZWN/iscqml9SEtzu4WSUWT4Ux32AgovF9HJBraAgf9UEfGb1AZ7XlaTZvfKttiT7
5NS+Wg2oX7jelcVuHPX82Jzg+luTSHDTmB8CCYb+pdXZ3n/CI5zoPvKF2uWRL7Cj
2rr0jYhKPceU5wDP2eNuGe9qCVgnYElqqQOYg4gzrCS6Yzdzby8Q3wZ52CkBYqWJ
SyUkEpxrDPCVRYX+G4ITdq7CgHfYQWA7in9AQ+Vm+bLz6OBfCuwKPCLPSi3nWQyC
DeeKSzHyBKh5n6mvV8Zu1zLsI3Sq4lkqiMLkDPb7ZzHdRO2p4P4vZQ==
-----END CERTIFICATE-----
`
